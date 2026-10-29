package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func getID(c *gin.Context) string {
	return strings.TrimPrefix(c.Param("id"), "/")
}

// dbError logs the real database error and answers with a generic message,
// so internal table/column/constraint names never reach the client.
func dbError(c *gin.Context, err error) {
	log.Printf("db error: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "A database error occurred"})
}

// bindError logs the real bind error and answers with a generic message,
// so internal Go struct/field names never reach the client.
func bindError(c *gin.Context, err error) {
	log.Printf("bind error: %v", err)
	c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
}
