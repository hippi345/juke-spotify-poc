package paidskip

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"juke-spotify-poc/server/auth"
	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
	"juke-spotify-poc/server/voting"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"
)

// Handlers serves paid-skip checkout and Stripe webhooks.
type Handlers struct {
	Manager *voting.Manager
	Config  *config.Config
}

// ConfigInfo exposes public paid-skip settings for clients.
func (h *Handlers) ConfigInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"price_usd":       PriceUSD,
		"currency":        Currency,
		"stripe_enabled":  h.Config.StripeSecretKey != "",
		"product_name":    ProductName,
	})
}

// CreateCheckout starts a Stripe Checkout session (test mode) for a playlist track.
func (h *Handlers) CreateCheckout(c *gin.Context) {
	if h.Config.StripeSecretKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Stripe is not configured (set STRIPE_SECRET_KEY)"})
		return
	}
	if auth.RoleFromContext(c) != "patron" {
		c.JSON(http.StatusForbidden, gin.H{"error": "patron login required"})
		return
	}
	patronID := auth.UserIDFromContext(c)
	if patronID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var body struct {
		TrackID string `json:"track_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "track_id required"})
		return
	}
	trackID := strings.TrimSpace(body.TrackID)
	if trackID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "track_id required"})
		return
	}

	sessionID := h.Manager.ActiveSessionID()
	if sessionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no active session"})
		return
	}

	var user models.User
	if err := db.DB.First(&user, patronID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	if user.JoinedVotingSessionID == nil || *user.JoinedVotingSessionID != sessionID {
		c.JSON(http.StatusForbidden, gin.H{"error": "join the venue session before paying to skip"})
		return
	}

	trackURI, err := h.Manager.PlaylistTrackURI(trackID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "track not on venue playlist"})
		return
	}

	stripe.Key = h.Config.StripeSecretKey
	successURL := strings.TrimRight(h.Config.AppBaseURL, "/") + "/paid-skip/success?session_id={CHECKOUT_SESSION_ID}"
	cancelURL := strings.TrimRight(h.Config.AppBaseURL, "/") + "/paid-skip/cancel"

	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(Currency),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(ProductName),
					},
					UnitAmount: stripe.Int64(PriceCents),
				},
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		Metadata: map[string]string{
			"voting_session_id": fmtUint(sessionID),
			"track_id":          trackID,
			"track_uri":         trackURI,
			"patron_user_id":    fmtUint(patronID),
		},
	}

	cs, err := session.New(params)
	if err != nil {
		log.Printf("paidskip: checkout create failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not create checkout session"})
		return
	}

	record := &models.PaidSkip{
		VotingSessionID:         sessionID,
		PatronUserID:            patronID,
		TrackID:                 trackID,
		TrackURI:                trackURI,
		StripeCheckoutSessionID: cs.ID,
		Status:                  "pending",
	}
	if err := db.DB.Create(record).Error; err != nil {
		log.Printf("paidskip: save pending skip: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not record checkout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"checkout_url": cs.URL,
		"session_id":   cs.ID,
		"price_usd":    PriceUSD,
	})
}

// StripeWebhook handles Stripe test-mode payment events.
func (h *Handlers) StripeWebhook(c *gin.Context) {
	if h.Config.StripeWebhookSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "webhook not configured"})
		return
	}
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body"})
		return
	}

	event, err := webhook.ConstructEvent(payload, c.GetHeader("Stripe-Signature"), h.Config.StripeWebhookSecret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signature"})
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		var cs stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "parse session"})
			return
		}
		if err := h.fulfillCheckoutSession(&cs); err != nil {
			log.Printf("paidskip: fulfill failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	default:
		// ignore other events
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

func (h *Handlers) fulfillCheckoutSession(cs *stripe.CheckoutSession) error {
	if cs.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		return nil
	}

	var skip models.PaidSkip
	if err := db.DB.Where("stripe_checkout_session_id = ?", cs.ID).First(&skip).Error; err != nil {
		return err
	}
	if skip.Status == "paid" {
		return nil
	}

	trackID := cs.Metadata["track_id"]
	trackURI := cs.Metadata["track_uri"]
	if trackID == "" || trackURI == "" {
		return fmt.Errorf("missing metadata")
	}

	// Re-validate playlist membership at fulfillment time.
	validatedURI, err := h.Manager.PlaylistTrackURI(trackID)
	if err != nil {
		skip.Status = "failed"
		db.DB.Save(&skip)
		return err
	}
	trackURI = validatedURI
	skip.TrackURI = trackURI

	now := time.Now()
	skip.Status = "paid"
	skip.PaidAt = &now
	if err := db.DB.Save(&skip).Error; err != nil {
		return err
	}

	if err := h.Manager.ApplyPaidSkipAfterPayment(&skip); err != nil {
		log.Printf("paidskip: queue after payment: %v", err)
		// Payment is recorded; advance round may still pick up queued_at NULL rows.
	}
	return nil
}

func fmtUint(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
