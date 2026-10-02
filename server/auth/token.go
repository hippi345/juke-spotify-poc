package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims carried in API bearer tokens.
type Claims struct {
	UserID uint   `json:"uid"`
	Role   string `json:"role"`
	Exp    int64  `json:"exp"`
}

// IssueToken signs claims with secret.
func IssueToken(secret string, userID uint, role string, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", errors.New("auth secret not configured")
	}
	claims := Claims{
		UserID: userID,
		Role:   role,
		Exp:    time.Now().Add(ttl).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(enc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return enc + "." + sig, nil
}

// ParseToken validates signature and expiry.
func ParseToken(secret, token string) (*Claims, error) {
	if secret == "" {
		return nil, errors.New("auth secret not configured")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid token")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return nil, errors.New("invalid token signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid token payload: %w", err)
	}
	var claims Claims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, err
	}
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}
	return &claims, nil
}
