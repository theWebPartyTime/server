package handlers

import (
	"net/http"
	"server/internal/models"
	"server/internal/service"
	"time"

	"github.com/gin-gonic/gin"
	go_jwt "github.com/golang-jwt/jwt/v5"
)

type AuthHandler struct {
	authService *service.AuthService
	secretKey   []byte
}

const (
	accessTTL  = time.Hour
	refreshTTL = 7 * 24 * time.Hour
)

func NewAuthHandler(authService *service.AuthService, secretKey []byte) *AuthHandler {
	return &AuthHandler{authService: authService, secretKey: secretKey}
}

func (h *AuthHandler) GetAuthService() *service.AuthService {
	return h.authService
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	accessToken, err := h.generateToken(user, accessTTL, "access")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
		return
	}

	refreshToken, err := h.generateToken(user, refreshTTL, "refresh")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
		},
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accessToken, err := h.generateToken(user, accessTTL, "access")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
		return
	}

	refreshToken, err := h.generateToken(user, refreshTTL, "refresh")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
		},
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := go_jwt.Parse(req.RefreshToken, func(t *go_jwt.Token) (interface{}, error) {
		if t.Method != go_jwt.SigningMethodHS256 {
			return nil, go_jwt.ErrSignatureInvalid
		}
		return h.secretKey, nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	claims, ok := token.Claims.(go_jwt.MapClaims)
	if !ok || claims["typ"] != "refresh" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
		return
	}

	user := &models.User{
		ID:    int(claims["id"].(float64)),
		Email: claims["email"].(string),
	}

	accessToken, err := h.generateToken(user, accessTTL, "access")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": accessToken,
	})
}

func (h *AuthHandler) generateToken(user *models.User, ttl time.Duration, typ string) (string, error) {
	now := time.Now()
	claims := go_jwt.MapClaims{
		"id":    user.ID,
		"email": user.Email,
		"iat":   now.Unix(),
		"exp":   now.Add(ttl).Unix(),
		"typ":   typ,
	}
	token := go_jwt.NewWithClaims(go_jwt.SigningMethodHS256, claims)
	return token.SignedString(h.secretKey)
}
