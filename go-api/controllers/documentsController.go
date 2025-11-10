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

// Mala pomoćna funkcija da ne dupliramo kod
func getSummaryFromPythonService(c *gin.Context) (*http.Response, []byte, error) {
	file, err := c.FormFile("file")
	if err != nil {
		return nil, nil, err
	}
	src, err := file.Open()
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}
	responseBody, err := io.ReadAll(resp.Body)
	return resp, responseBody, err
}

func PublicSummarize(c *gin.Context) {
	resp, body, err := getSummaryFromPythonService(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process summary request"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "AI service returned an error", "details": string(body)})
		return
	}
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func CreateSummary(c *gin.Context) {
	resp, body, err := getSummaryFromPythonService(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process summary request"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "AI service returned an error", "details": string(body)})
		return
	}

	var summaryResponse struct {
		Filename string `json:"filename"`
		Summary  string `json:"summary"`
	}
	json.Unmarshal(body, &summaryResponse)
	userInterface, _ := c.Get("user")
	user := userInterface.(models.User)

	document := models.Document{
		Filename: summaryResponse.Filename,
		Summary:  summaryResponse.Summary,
		UserID:   user.ID,
	}
	initializers.DB.Create(&document)

	c.JSON(http.StatusOK, summaryResponse)
}

func ListDocuments(c *gin.Context) {
	user, _ := c.Get("user")

	var documents []models.Document
	initializers.DB.Where("user_id = ?", user.(models.User).ID).Find(&documents)

	c.JSON(http.StatusOK, documents)
}