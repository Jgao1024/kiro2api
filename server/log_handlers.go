package server

import (
	"net/http"
	"strconv"

	"kiro2api/record"

	"github.com/gin-gonic/gin"
)

func handleLogsAPI(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "12"))
	if page < 1 {
		page = 1
	}

	phase := c.DefaultQuery("phase", "")
	requestID := c.DefaultQuery("request_id", "")
	list, total, err := record.QueryLogs(page, size, phase, requestID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []record.LogRow{}
	}
	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"page":  page,
		"size":  size,
		"data":  list,
	})
}

func handleLogBodyAPI(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	headers, body, err := record.GetBody(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"headers": headers, "body": body})
}
