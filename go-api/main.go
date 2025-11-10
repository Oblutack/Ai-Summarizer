package main

import (
	"ai-summarizer/go-api/controllers"
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/middleware"
	"ai-summarizer/go-api/models"
	"fmt"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func init() {
	initializers.ConnectToDB()
}

func main() {
	initializers.DB.AutoMigrate(&models.User{}, &models.Document{})
	fmt.Println("✅ Database migration completed!")

	r := gin.Default()

    config := cors.DefaultConfig()
    config.AllowOrigins = []string{"http://localhost:3000"} 
    config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE"} 
    config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"} 
    r.Use(cors.New(config))


	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Hello from Go API Gateway"})
	})
	r.POST("/signup", controllers.Signup)
	r.POST("/login", controllers.Login)
	r.POST("/public/summarize", controllers.PublicSummarize)
	r.POST("/public/summarize-text", controllers.PublicSummarizeText)

	authorized := r.Group("/")
	authorized.Use(middleware.RequireAuth)
	{
		authorized.POST("/summarize", controllers.CreateSummary)
		authorized.GET("/documents", controllers.ListDocuments)
		authorized.POST("/summarize-text", controllers.CreateSummaryText)
	}

	r.Run(":8080")
}