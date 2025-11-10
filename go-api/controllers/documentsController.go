package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TextPayload struct {
	Text string `json:"text"`
}

// Mala pomoćna funkcija da ne dupliramo kod
func getSummaryFromPythonService(c *gin.Context) (*http.Response, []byte, error) {
	wordCount := c.PostForm("wordCount") // Čita wordCount iz forme
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

	if err := writer.WriteField("word_count", wordCount); err != nil {
		return nil, nil, err
	}

	part, err := writer.CreateFormFile("file", file.Filename)
	if err != nil {
		return nil, nil, err
	}

	if _, err := io.Copy(part, src); err != nil {
		return nil, nil, err
	}
	
	writer.Close() // Zatvaramo writer pre slanja

	pythonServiceURL := "http://python-ai-service:8000/summarize"
	req, err := http.NewRequest("POST", pythonServiceURL, &requestBody)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	
	responseBody, err := io.ReadAll(resp.Body)
	return resp, responseBody, err
}

func getSummaryFromPythonServiceForText(c *gin.Context) (*http.Response, []byte, error) {
	var payload TextPayload
	if err := c.BindJSON(&payload); err != nil {
		return nil, nil, err
	}
	
	wordCount := c.Query("wordCount") // Čita wordCount iz URL query parametra

	jsonBody, _ := json.Marshal(payload)

	// Dodajemo word_count u URL
	pythonServiceURL := fmt.Sprintf("http://python-ai-service:8000/summarize-text?word_count=%s", wordCount)

	req, _ := http.NewRequest("POST", pythonServiceURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	responseBody, err := io.ReadAll(resp.Body)
	return resp, responseBody, err
}

func PublicSummarizeText(c *gin.Context) {
	resp, body, err := getSummaryFromPythonServiceForText(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process text summary request"})
		return
	}
	defer resp.Body.Close()
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func CreateSummaryText(c *gin.Context) {
	resp, body, err := getSummaryFromPythonServiceForText(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process text summary request"})
		return
	}
	defer resp.Body.Close()

	var summaryResponse struct {
		Summary string `json:"summary"`
	}
	json.Unmarshal(body, &summaryResponse)
	userInterface, _ := c.Get("user")
	user := userInterface.(models.User)

	document := models.Document{
		Filename: "Pasted Text", 
		Summary:  summaryResponse.Summary,
		UserID:   user.ID,
	}
	initializers.DB.Create(&document)

	c.JSON(http.StatusOK, summaryResponse)
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