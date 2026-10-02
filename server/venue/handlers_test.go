package venue

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"juke-spotify-poc/server/auth"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupVenueTest(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&models.User{}, &models.Venue{}, &models.VotingSession{}); err != nil {
		t.Fatal(err)
	}
	db.SetDB(database)

	secret := "test-auth-secret"
	r := gin.New()
	r.Use(auth.Middleware(secret))
	authH := &auth.Handlers{Secret: secret}
	venueH := &Handlers{}
	r.POST("/api/auth/register", authH.Register)
	r.POST("/api/venues", venueH.Create)
	r.GET("/api/venues/nearby", venueH.Nearby)
	r.POST("/api/venues/:id/join", venueH.Join)

	// staff
	body := map[string]string{"email": "staff@example.com", "password": "password123", "role": "staff"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("staff register: %d %s", w.Code, w.Body.String())
	}
	var regResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &regResp)
	staffToken := regResp["token"].(string)

	// create venue
	venueBody := map[string]interface{}{"name": "Test Bar", "latitude": 40.7128, "longitude": -74.006}
	b2, _ := json.Marshal(venueBody)
	req2 := httptest.NewRequest(http.MethodPost, "/api/venues", bytes.NewReader(b2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+staffToken)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("create venue: %d %s", w2.Code, w2.Body.String())
	}

	// active session at venue
	var venue models.Venue
	if err := database.First(&venue).Error; err != nil {
		t.Fatal(err)
	}
	session := models.VotingSession{
		VenueID:      &venue.ID,
		PlaylistID:   "pl",
		Status:       "active",
		RefillCount:  10,
		RefillThreshold: 0,
	}
	if err := database.Create(&session).Error; err != nil {
		t.Fatal(err)
	}

	// patron
	body3 := map[string]string{"email": "patron@example.com", "password": "password123", "role": "patron"}
	b3, _ := json.Marshal(body3)
	req3 := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(b3))
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	patronToken := jsonUnmarshalToken(w3.Body.Bytes())

	return r, patronToken
}

func jsonUnmarshalToken(raw []byte) string {
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return m["token"].(string)
}

func TestNearbyAndJoin(t *testing.T) {
	r, patronToken := setupVenueTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/venues/nearby?lat=40.7128&lng=-74.006&radius_m=5000", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("nearby: %d %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/venues/1/join", bytes.NewReader([]byte("{}")))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+patronToken)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("join: %d %s", w2.Code, w2.Body.String())
	}
}
