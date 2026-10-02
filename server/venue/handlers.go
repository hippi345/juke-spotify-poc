package venue

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"juke-spotify-poc/server/auth"
	"juke-spotify-poc/server/cache"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
	"juke-spotify-poc/server/search"

	"github.com/gin-gonic/gin"
)

// Handlers for venue CRUD, nearby discovery, and patron join.
type Handlers struct{}

func (h *Handlers) Create(c *gin.Context) {
	if !auth.RequireRole(c, "staff") {
		return
	}
	staffID := auth.UserIDFromContext(c)
	var body struct {
		Name      string  `json:"name" binding:"required"`
		Latitude  float64 `json:"latitude" binding:"required"`
		Longitude float64 `json:"longitude" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, latitude, and longitude required"})
		return
	}
	v := models.Venue{
		StaffUserID: staffID,
		Name:        strings.TrimSpace(body.Name),
		Latitude:    body.Latitude,
		Longitude:   body.Longitude,
	}
	if v.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	if err := db.DB.Create(&v).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create venue"})
		return
	}
	_ = search.Default.IndexVenue(c.Request.Context(), search.VenueHit{
		ID:        v.ID,
		Name:      v.Name,
		Latitude:  v.Latitude,
		Longitude: v.Longitude,
		StaffID:   v.StaffUserID,
	})
	c.JSON(http.StatusCreated, venueResponse(&v))
}

// Search finds venues by name (OpenSearch when configured, otherwise SQL fallback).
func (h *Handlers) Search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q query param required"})
		return
	}
	limit := 20
	if hits, err := search.Default.SearchVenues(c.Request.Context(), q, limit); err == nil && len(hits) > 0 {
		out := make([]gin.H, 0, len(hits))
		for _, hit := range hits {
			out = append(out, gin.H{
				"id":            hit.ID,
				"name":          hit.Name,
				"latitude":      hit.Latitude,
				"longitude":     hit.Longitude,
				"staff_user_id": hit.StaffID,
			})
		}
		c.JSON(http.StatusOK, gin.H{"venues": out, "source": "opensearch"})
		return
	}
	var venues []models.Venue
	if err := db.DB.Where("name LIKE ?", "%"+q+"%").Limit(limit).Find(&venues).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not search venues"})
		return
	}
	out := make([]gin.H, 0, len(venues))
	for _, v := range venues {
		out = append(out, venueResponse(&v))
	}
	c.JSON(http.StatusOK, gin.H{"venues": out, "source": "mysql"})
}

func (h *Handlers) Mine(c *gin.Context) {
	if !auth.RequireRole(c, "staff") {
		return
	}
	staffID := auth.UserIDFromContext(c)
	var venues []models.Venue
	if err := db.DB.Where("staff_user_id = ?", staffID).Order("id asc").Find(&venues).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list venues"})
		return
	}
	out := make([]gin.H, 0, len(venues))
	for _, v := range venues {
		out = append(out, venueResponse(&v))
	}
	c.JSON(http.StatusOK, gin.H{"venues": out})
}

func (h *Handlers) Nearby(c *gin.Context) {
	lat, err1 := strconv.ParseFloat(c.Query("lat"), 64)
	lng, err2 := strconv.ParseFloat(c.Query("lng"), 64)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat and lng query params required"})
		return
	}
	radiusM := 2000.0
	if r := c.Query("radius_m"); r != "" {
		if parsed, err := strconv.ParseFloat(r, 64); err == nil && parsed > 0 {
			radiusM = parsed
		}
	}

	cacheKey := fmt.Sprintf("jukespotify:nearby:%.5f:%.5f:%.0f", lat, lng, radiusM)
	if raw, ok := cache.Default.Get(c.Request.Context(), cacheKey); ok {
		c.Header("X-Cache", "HIT")
		c.Data(http.StatusOK, "application/json", []byte(raw))
		return
	}
	c.Header("X-Cache", "MISS")

	var venues []models.Venue
	if err := db.DB.Find(&venues).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load venues"})
		return
	}

	type nearbyRow struct {
		venue    models.Venue
		session  models.VotingSession
		distance float64
	}
	results := make([]nearbyRow, 0)
	for _, v := range venues {
		var session models.VotingSession
		err := db.DB.Where("venue_id = ? AND status = ?", v.ID, "active").Order("id desc").First(&session).Error
		if err != nil {
			continue
		}
		d := HaversineDistanceMeters(lat, lng, v.Latitude, v.Longitude)
		if d > radiusM {
			continue
		}
		results = append(results, nearbyRow{venue: v, session: session, distance: d})
	}

	out := make([]gin.H, 0, len(results))
	for _, row := range results {
		joinProtected := row.session.JoinPasswordHash != ""
		out = append(out, gin.H{
			"venue":              venueResponse(&row.venue),
			"session_id":         row.session.ID,
			"playlist_name":      row.session.PlaylistName,
			"distance_m":         int(row.distance),
			"requires_password":  joinProtected,
		})
	}
	resp := gin.H{"sessions": out}
	raw, _ := json.Marshal(resp)
	_ = cache.Default.Set(c.Request.Context(), cacheKey, string(raw), 30*time.Second)
	c.Header("X-Cache", "MISS")
	c.Data(http.StatusOK, "application/json", raw)
}

func (h *Handlers) Join(c *gin.Context) {
	if !auth.RequireRole(c, "patron") {
		return
	}
	patronID := auth.UserIDFromContext(c)
	venueID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || venueID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid venue id"})
		return
	}
	var venue models.Venue
	if err := db.DB.First(&venue, venueID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "venue not found"})
		return
	}
	var session models.VotingSession
	if err := db.DB.Where("venue_id = ? AND status = ?", venue.ID, "active").Order("id desc").First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active session at this venue"})
		return
	}

	var body struct {
		JoinPassword string `json:"join_password"`
	}
	_ = c.ShouldBindJSON(&body)

	if session.JoinPasswordHash != "" {
		if body.JoinPassword == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "join password required"})
			return
		}
		if !auth.CheckPassword(session.JoinPasswordHash, body.JoinPassword) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "incorrect join password"})
			return
		}
	}

	if err := db.DB.Model(&models.User{}).Where("id = ?", patronID).
		Update("joined_voting_session_id", session.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not join session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "joined",
		"venue":      venueResponse(&venue),
		"session_id": session.ID,
	})
}

func venueResponse(v *models.Venue) gin.H {
	return gin.H{
		"id":         v.ID,
		"name":       v.Name,
		"latitude":   v.Latitude,
		"longitude":  v.Longitude,
		"staff_user_id": v.StaffUserID,
	}
}
