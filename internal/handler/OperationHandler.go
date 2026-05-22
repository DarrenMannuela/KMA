package handler

import (
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetOperation(c *gin.Context) {
	var Operations []dto.Operations

	db := Connect()
	results := db.Find(&Operations)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
	}

	c.JSON(200, Operations)

}
func PostOperation(c *gin.Context) {
	var newOperation dto.Operations

	db := Connect()
	if err := c.ShouldBindBodyWithJSON(&newOperation); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}
	results := db.Create(&newOperation)

	if results.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
	}
	c.JSON(201, newOperation)

}

func UpdateOperation(c *gin.Context) {
	id := c.Param("id")
	var updateOperation dto.Operations
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&updateOperation); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}

	if err := db.First(&updateOperation, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
	}
	db.Save(&updateOperation)
	c.JSON(http.StatusOK, updateOperation)

}

func DeleteOperation(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Delete(&dto.Operations{}, id)

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Operation not found"})
	}

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
	}

	c.Status(http.StatusNoContent)
}
