package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"fmt"
	"net/http"
	"os"
	"time"

	"context"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

func Signup(c *gin.Context) {
	var body struct {
		Email    string
		Password string
	}

	if c.Bind(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 10)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{Email: body.Email, Password: string(hash)}
	result := initializers.DB.Create(&user)

	if result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func Login(c *gin.Context) {
	var body struct {
		Email    string
		Password string
	}

	if c.Bind(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	var user models.User
	initializers.DB.First(&user, "email = ?", body.Email)

	if user.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email or password"})
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(body.Password))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email or password"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(time.Hour * 24 * 30).Unix(), 
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("SECRET")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
	})
}

func GoogleLogin(c *gin.Context) {
	var body struct {
		Token string `json:"token"`
	}

	if err := c.BindJSON(&body); err != nil {
		fmt.Println("!!! ERROR binding JSON:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	fmt.Println("--- Received Google Token from frontend ---")


	// UPOZORENJE: Ovo je haos za produkciju! Koristi se samo za rješavanje
	// problema sa TLS sertifikatima u lokalnom Docker okruženju.
	oauth2Service, err := oauth2.NewService(context.Background(), option.WithoutAuthentication())
	if err != nil {
		fmt.Println("!!! ERROR creating Google service:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to init Google service"})
		return
	}
	
	tokenInfo, err := oauth2Service.Tokeninfo().IdToken(body.Token).Do()
	if err != nil {
		fmt.Println("!!! ERROR validating token with Google:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Google token"})
		return
	}

	fmt.Println("--- Google Token is Valid, Email:", tokenInfo.Email, "---")

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if tokenInfo.Audience != googleClientID {
		fmt.Println("!!! ERROR token audience mismatch.")
		fmt.Println("   > Token Audience from Google:", tokenInfo.Audience)
		fmt.Println("   > Expected Audience from .env:", googleClientID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is not for this app"})
		return
	}

	var user models.User
	initializers.DB.First(&user, "email = ?", tokenInfo.Email)

	if user.ID == 0 {
		fmt.Println("--- User not found, creating new user... ---")
		randomPassword := time.Now().String() 
		hash, _ := bcrypt.GenerateFromPassword([]byte(randomPassword), 10) 
		
		user = models.User{Email: tokenInfo.Email, Password: string(hash)}
		result := initializers.DB.Create(&user)
		if result.Error != nil {
			fmt.Println("!!! ERROR creating user in DB:", result.Error)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
	}

	fmt.Println("--- User found or created, ID:", user.ID, "---")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(time.Hour * 24 * 30).Unix(),
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("SECRET")))
	if err != nil {
		fmt.Println("!!! ERROR creating local JWT:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create token"})
		return
	}

	fmt.Println("--- Successfully created and sent local JWT ---")
	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}