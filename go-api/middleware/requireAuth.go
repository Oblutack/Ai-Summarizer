package middleware

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func RequireAuth(c *gin.Context) {
	scheme, tokenString, ok := strings.Cut(c.GetHeader("Authorization"), " ")
	if !ok || scheme != "Bearer" || tokenString == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// jwt.Parse also validates the exp claim.
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("SECRET")), nil
	}, jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	sub, ok := claims["sub"].(float64)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var user models.User
	if err := initializers.DB.First(&user, uint(sub)).Error; err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.Set("user", user)
	c.Next()
}
