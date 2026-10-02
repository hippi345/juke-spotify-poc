package paidskip

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
	"juke-spotify-poc/server/voting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPaidSkipTest(t *testing.T) (*gin.Engine, *voting.Manager) {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := database.AutoMigrate(&models.User{}, &models.VotingSession{}, &models.PaidSkip{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db.SetDB(database)

	gin.SetMode(gin.TestMode)
	mgr := voting.NewManager(nil)
	cfg := &config.Config{AppBaseURL: "http://localhost:5173"}
	h := &Handlers{Manager: mgr, Config: cfg}

	r := gin.New()
	r.GET("/api/paid-skip/config", h.ConfigInfo)
	r.POST("/api/paid-skip/checkout", func(c *gin.Context) {
		c.Set("auth_user_id", uint(1))
		c.Set("auth_role", "patron")
		h.CreateCheckout(c)
	})
	return r, mgr
}

func TestConfigInfo(t *testing.T) {
	r, _ := setupPaidSkipTest(t)
	req := httptest.NewRequest(http.MethodGet, "/api/paid-skip/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"price_usd":"1.00"`) {
		t.Fatalf("unexpected body %s", w.Body.String())
	}
}

func TestCreateCheckout_requiresStripe(t *testing.T) {
	r, _ := setupPaidSkipTest(t)
	req := httptest.NewRequest(http.MethodPost, "/api/paid-skip/checkout", strings.NewReader(`{"track_id":"abc"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestPriceConstants(t *testing.T) {
	if PriceCents != 100 || PriceUSD != "1.00" {
		t.Fatalf("unexpected price constants")
	}
}
