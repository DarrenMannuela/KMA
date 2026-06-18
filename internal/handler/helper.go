package handler

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

func getID(c *gin.Context) string {
	id := c.Param("id")
	if strings.Contains(id, "%") {
		decoded, err := url.QueryUnescape(id)
		if err == nil {
			return decoded
		}
	}
	return strings.TrimPrefix(id, "/")
}
