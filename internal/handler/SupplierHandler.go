package handler

import (
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetSupplier(c *gin.Context) {
	var suppliers []dto.Supplier
	db := Connect()

	results := db.Find(&suppliers)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
	}
	c.JSON(200, suppliers)
	return
}

func SetSupplier(c *gin.Context) {
	var supplier dto.Supplier
	db := Connect()

	if err := c.ShouldBindJSON(&supplier); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
	}

	results := db.Create(&supplier)

	if results.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
	}

	c.JSON(http.StatusCreated, supplier)
}

func GetSupplierByID(c *gin.Context) {
	id := c.Param("id")
	var supplier dto.Supplier

	db := Connect()

	result := db.First(&supplier, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
	}

	c.JSON(http.StatusOK, supplier)

}

func UpdateSupplier(c *gin.Context) {
	id := c.Param("id")
	var supplier dto.Supplier

	db := Connect()
	// Check if it exists first
	if err := db.First(&supplier, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
	}

	// Bind new data
	if err := c.ShouldBindJSON(&supplier); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}

	db.Save(&supplier)
	c.JSON(http.StatusOK, supplier)

}

func DeleteSupplier(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Delete(&dto.Supplier{}, id)

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
	}

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
	}

	c.Status(http.StatusNoContent)
}
