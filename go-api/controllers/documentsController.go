package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CreateSummary(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer src.Close()
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	part, _ := writer.CreateFormFile("file", file.Filename)
	io.Copy(part, src)
	writer.Close()
	pythonServiceURL := "http://python-ai-service:8000/summarize"
	req, _ := http.NewRequest("POST", pythonServiceURL, &requestBody)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI service is unavailable"})
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read AI service response"})
		return
	}
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "AI service returned an error", "details": string(responseBody)})
		return
	}

	var summaryResponse struct {
		Filename string `json:"filename"`
		Summary  string `json:"summary"`
	}
	json.Unmarshal(responseBody, &summaryResponse)

	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusOK, summaryResponse)
		return
	}

	user, ok := userInterface.(models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user"})
		return
	}

	document := models.Document{
		Filename: summaryResponse.Filename,
		Summary:  summaryResponse.Summary,
		UserID:   user.ID, 
	}
	result := initializers.DB.Create(&document)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save document to database"})
		return
	}

	c.JSON(http.StatusOK, summaryResponse)
}

func ListDocuments(c *gin.Context) {
	user, _ := c.Get("user")

	var documents []models.Document
	initializers.DB.Where("user_id = ?", user.(models.User).ID).Find(&documents)

	c.JSON(http.StatusOK, documents)
}