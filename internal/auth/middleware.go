package auth

import (
	"net/http"
	"server/internal/models"
	"strconv"
	"strings"

	"github.com/centrifugal/centrifuge"
	"github.com/gin-gonic/gin"
	go_jwt "github.com/golang-jwt/jwt/v5"
)

type JWTMiddleware struct {
	SecretKey []byte
}

func NewJWTMiddleware(secretKey []byte) *JWTMiddleware {
	return &JWTMiddleware{SecretKey: secretKey}
}

func (m *JWTMiddleware) GinAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := m.parseToken(c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

func (m *JWTMiddleware) parseToken(authHeader string) (*models.User, error) {
	if authHeader == "" {
		return nil, http.ErrNoCookie
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, http.ErrNoCookie
	}

	tokenStr := parts[1]

	token, err := go_jwt.Parse(tokenStr, func(t *go_jwt.Token) (interface{}, error) {
		if t.Method != go_jwt.SigningMethodHS256 {
			return nil, go_jwt.ErrSignatureInvalid
		}
		return m.SecretKey, nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}

	claims, ok := token.Claims.(go_jwt.MapClaims)
	if !ok || claims["typ"] != "access" {
		return nil, http.ErrNoCookie
	}

	user := &models.User{
		ID:    int(claims["id"].(float64)),
		Email: claims["email"].(string),
	}

	return user, nil
}

func (m *JWTMiddleware) WSAuthMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		user, err := m.parseToken(r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := centrifuge.SetCredentials(
			r.Context(),
			&centrifuge.Credentials{
				UserID: strconv.Itoa(user.ID),
			},
		)

		r = r.WithContext(ctx)
		h.ServeHTTP(w, r)
	})
}

// func (m *JWTMiddleware) WSIdentityMiddleware(h http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

// 		var userID string

// 		username := r.Header.Get("X-Username")
// 		if username == "" {
// 				http.Error(w, "username is required", http.StatusUnauthorized)
// 				return
// 			}

// 		ctx := centrifuge.SetCredentials(
// 			r.Context(),
// 			&centrifuge.Credentials{
// 				UserID: userID,
// 			},
// 		)

// 		r = r.WithContext(ctx)
// 		h.ServeHTTP(w, r)
// 	})
// }
