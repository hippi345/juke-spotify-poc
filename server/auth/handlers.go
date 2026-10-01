package auth

import (
	"net/http"
	"strings"
	"time"

	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"

	"github.com/gin-gonic/gin"
)

const tokenTTL = 7 * 24 * time.Hour

// Handlers serves register/login/me.
type Handlers struct {
	Secret string
}

func (h *Handlers) Register(c *gin.Context) {
	var body struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email, password, and role required"})
		return
	}
	role := strings.ToLower(strings.TrimSpace(body.Role))
	if role != "staff" && role != "patron" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be staff or patron"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if email == "" || len(body.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email and password (min 8 chars) required"})
		return
	}
	hash, err := HashPassword(body.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hash password"})
		return
	}
	user := models.User{Email: email, PasswordHash: hash, Role: role}
	if err := db.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}
	token, err := IssueToken(h.Secret, user.ID, user.Role, tokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user":  userResponse(&user),
	})
}

func (h *Handlers) Login(c *gin.Context) {
	var body struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password required"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	var user models.User
	if err := db.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if !CheckPassword(user.PasswordHash, body.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	token, err := IssueToken(h.Secret, user.ID, user.Role, tokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  userResponse(&user),
	})
}

func (h *Handlers) Me(c *gin.Context) {
	uid := UserIDFromContext(c)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	var user models.User
	if err := db.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userResponse(&user)})
}

func userResponse(u *models.User) gin.H {
	return gin.H{
		"id":                       u.ID,
		"email":                    u.Email,
		"role":                     u.Role,
		"joined_voting_session_id": u.JoinedVotingSessionID,
	}
}
