package main

import (
	"fmt"
	"net/http"

	"ai-summarizer/go-api/controllers"
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/middleware"
	"ai-summarizer/go-api/models"

	"github.com/gin-gonic/gin"
)

func init() {
	initializers.ConnectToDB() 
}

func main() {
	
	initializers.DB.AutoMigrate(&models.User{}, &models.Document{})
    fmt.Println("Database migration completed!")
	
	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello from Go API Gateway",
		})
	})

	r.POST("/signup", controllers.Signup)
	r.POST("/login", controllers.Login)

	authorized := r.Group("/")

	authorized.Use(middleware.RequireAuth)
	{
		authorized.POST("/summarize", controllers.CreateSummary)
		
		authorized.GET("/documents", controllers.ListDocuments)
	}
	

	r.Run(":8080")
}