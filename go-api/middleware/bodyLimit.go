package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxBody caps the request body at maxBytes; larger bodies fail when read.
func MaxBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "Request is too large.",
			})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
