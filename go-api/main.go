package main

import (
	"ai-summarizer/go-api/controllers"
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/middleware"
	"ai-summarizer/go-api/models"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// listenAddr honors PORT (set by hosts like Render) and defaults to 8080.
func listenAddr() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return ":8080"
}

// runHealthcheck probes the local server and exits 0/1. It exists so the minimal scratch-based
// image, which has no shell or curl, can still define a container HEALTHCHECK.
func runHealthcheck() {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1" + listenAddr() + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
	os.Exit(0)
}

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the running server and exit")
	flag.Parse()
	if *healthcheck {
		runHealthcheck()
	}

	initializers.ConnectToDB()
	if err := initializers.DB.AutoMigrate(&models.User{}, &models.Document{}); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}
	log.Println("database migration completed")

	r := gin.Default()

	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:3000", "https://ai-summarizer-ten-tan.vercel.app"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	r.Use(cors.New(config))

	// Summaries are expensive LLM calls, so they get a tighter budget than auth.
	summarizeLimit := middleware.RateLimit(middleware.NewRateLimiter(6, 3))
	authLimit := middleware.RateLimit(middleware.NewRateLimiter(20, 10))
	fileBody := middleware.MaxBody(controllers.MaxPDFBytes + 1<<20)
	multiBody := middleware.MaxBody(controllers.MaxMultiBytes + 1<<20)
	textBody := middleware.MaxBody(controllers.MaxTextBytes)
	chatBody := middleware.MaxBody(controllers.MaxTextBytes)
	chatLimit := middleware.RateLimit(middleware.NewRateLimiter(20, 10))

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Hello from Go API Gateway"})
	})
	r.GET("/healthz", controllers.Healthz)
	r.GET("/readyz", controllers.Readyz(controllers.DatabaseCheck(), controllers.AIServiceCheck()))
	r.GET("/options", controllers.Options)
	r.POST("/signup", authLimit, controllers.Signup)
	r.POST("/login", authLimit, controllers.Login)
	r.POST("/auth/google", authLimit, controllers.GoogleLogin)
	r.POST("/public/summarize", summarizeLimit, fileBody, controllers.PublicSummarize)
	r.POST("/public/summarize-multiple", summarizeLimit, multiBody, controllers.PublicSummarizeMultiple)
	r.POST("/public/summarize-text", summarizeLimit, textBody, controllers.PublicSummarizeText)

	authorized := r.Group("/")
	authorized.Use(middleware.RequireAuth)
	{
		authorized.POST("/summarize", summarizeLimit, fileBody, controllers.CreateSummary)
		authorized.POST("/summarize-multiple", summarizeLimit, multiBody, controllers.CreateSummaryMultiple)
		authorized.POST("/summarize-text", summarizeLimit, textBody, controllers.CreateSummaryText)
		authorized.POST("/documents/:id/chat", chatLimit, chatBody, controllers.ChatWithDocument)
		authorized.GET("/documents", controllers.ListDocuments)
		authorized.DELETE("/documents/:id", controllers.DeleteDocument)
	}

	if err := r.Run(listenAddr()); err != nil {
		log.Fatal(err)
	}
}
