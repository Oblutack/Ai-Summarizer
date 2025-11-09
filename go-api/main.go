package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello from Go API Gateway",
		})
	})

	r.POST("/summarize", func(c *gin.Context) {
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
		part, err := writer.CreateFormFile("file", file.Filename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create form file"})
			return
		}
		_, err = io.Copy(part, src)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy file content"})
			return
		}
		writer.Close()

		pythonServiceURL := "http://python-ai-service:8000/summarize"

		req, err := http.NewRequest("POST", pythonServiceURL, &requestBody)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
			return
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI service is unavailable"})
			return
		}
		defer resp.Body.Close()

		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
	})

	r.Run(":8080")
}