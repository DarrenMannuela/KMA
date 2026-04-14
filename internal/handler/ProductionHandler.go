package handler

import (
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetProduction(c *gin.Context) {
	var productions []dto.Production

	db := Connect()
	results := db.Find(&productions)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
	}

	c.JSON(200, productions)

}
func PostProduction(c *gin.Context) {
	var newProduction dto.Production

	db := Connect()
	if err := c.ShouldBindBodyWithJSON(&newProduction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}
	results := db.Create(&newProduction)

	if results.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
	}
	c.JSON(201, newProduction)

}

func UpdateProduction(c *gin.Context) {
	id := c.Param("id")
	var updateProduction dto.Production
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&updateProduction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}

	if err := db.First(&updateProduction, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
	}

}

func DeleteProduction(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Delete(&dto.Production{}, id)

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Production not found"})
	}

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
	}

	c.Status(http.StatusNoContent)
}
