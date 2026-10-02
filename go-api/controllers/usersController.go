package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
	"gorm.io/gorm"
)

const tokenLifetime = 7 * 24 * time.Hour

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func issueToken(userID uint) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(tokenLifetime).Unix(),
	})
	return token.SignedString([]byte(os.Getenv("SECRET")))
}

func Signup(c *gin.Context) {
	var body credentials
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	email, err := normalizeEmail(body.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validatePassword(body.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existing int64
	initializers.DB.Model(&models.User{}).Where("lower(email) = ?", email).Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "An account with this email already exists."})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{Email: email, Password: string(hash)}
	if err := initializers.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func Login(c *gin.Context) {
	var body credentials
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	var user models.User
	err := initializers.DB.First(&user, "lower(email) = ?", normalizeLoose(body.Email)).Error
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(body.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	tokenString, err := issueToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func GoogleLogin(c *gin.Context) {
	var body struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	oauth2Service, err := oauth2.NewService(context.Background(), option.WithoutAuthentication())
	if err != nil {
		slog.Error("google login: init service", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to init Google service"})
		return
	}

	tokenInfo, err := oauth2Service.Tokeninfo().IdToken(body.Token).Do()
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Google token"})
		return
	}
	if tokenInfo.Audience != os.Getenv("GOOGLE_CLIENT_ID") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token is not for this app"})
		return
	}
	if !tokenInfo.VerifiedEmail {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Google account email is not verified"})
		return
	}

	email, err := normalizeEmail(tokenInfo.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	err = initializers.DB.First(&user, "lower(email) = ?", email).Error
	if err == gorm.ErrRecordNotFound {
		// Google users never use a password; store an unguessable one.
		hash, herr := randomPasswordHash()
		if herr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
		user = models.User{Email: email, Password: hash}
		if err := initializers.DB.Create(&user).Error; err != nil {
			slog.Error("google login: create user", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
	} else if err != nil {
		slog.Error("google login: lookup user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to look up user"})
		return
	}

	tokenString, err := issueToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func randomPasswordHash() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(base64.RawURLEncoding.EncodeToString(raw)[:48]), bcrypt.DefaultCost)
	return string(hash), err
}
