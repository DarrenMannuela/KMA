package handler

import (
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetInvoice(c *gin.Context) {
	var invoices []dto.Invoice
	db := Connect()
	result := db.Find(&invoices)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(200, invoices)
}

func GetInvoiceByID(c *gin.Context) {
	id := getID(c)
	var invoice dto.Invoice
	db := Connect()
	if err := db.Where("id = ?", id).First(&invoice).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		return
	}
	c.JSON(200, invoice)
}

func PostInvoice(c *gin.Context) {
	var newInvoice dto.Invoice
	db := Connect()
	if err := c.ShouldBindBodyWithJSON(&newInvoice); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result := db.Create(&newInvoice)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(201, newInvoice)
}

func UpdateInvoice(c *gin.Context) {
	id := getID(c)
	var existing dto.Invoice
	db := Connect()
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		return
	}
	var body dto.Invoice
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	body.Id = existing.Id
	if err := db.Save(&body).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, body)
}

func DeleteInvoice(c *gin.Context) {
	id := getID(c)
	db := Connect()
	result := db.Where("id = ?", id).Delete(&dto.Invoice{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		return
	}
	c.Status(http.StatusNoContent)
}
