package handler

import (
	"net/http"
	"strings"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetSuratJalan(c *gin.Context) {
	var SuratJalans []dto.SuratJalan

	db := Connect()
	results := db.Find(&SuratJalans)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
	}

	c.JSON(200, SuratJalans)

}
func PostSuratJalan(c *gin.Context) {
	var newSuratJalan dto.SuratJalan

	db := Connect()
	if err := c.ShouldBindBodyWithJSON(&newSuratJalan); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}
	results := db.Create(&newSuratJalan)

	if results.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
	}
	c.JSON(201, newSuratJalan)

}

func UpdateSuratJalan(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	var updateSuratJalan dto.SuratJalan
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&updateSuratJalan); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}

	if err := db.First(&updateSuratJalan, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
	}
	db.Save(&updateSuratJalan)
	c.JSON(http.StatusOK, updateSuratJalan)

}

func DeleteSuratJalan(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.SuratJalan{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Production not found"})
	}

	c.Status(http.StatusNoContent)
}
