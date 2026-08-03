package handler

import (
	"encoding/json"
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

	// Same pattern as PostOrders: catch the common "two clients suggested
	// the same next invoice number" case with a clean pre-check, and
	// treat any Create failure past that point as the true-simultaneous
	// case hitting the Id primary key constraint — see PostOrders'
	// comment for the fuller reasoning.
	var conflict dto.Invoice
	if err := db.Where("id = ?", newInvoice.Id).First(&conflict).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "An invoice with this ID already exists"})
		return
	}

	result := db.Create(&newInvoice)
	if result.Error != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "An invoice with this ID already exists"})
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

	// raw tells us which fields the client actually sent — a PATCH with
	// only {"status": "paid"} must leave everything else (total, dates,
	// client details, etc.) untouched.
	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	var body dto.Invoice
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	newId := existing.Id
	if _, ok := raw["id"]; ok && body.Id != "" {
		newId = body.Id
	}
	if newId != existing.Id {
		var conflict dto.Invoice
		if err := db.Where("id = ?", newId).First(&conflict).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "An invoice with this ID already exists"})
			return
		}
	}

	updates := map[string]interface{}{"id": newId}
	if _, ok := raw["order_id"]; ok {
		updates["order_id"] = body.OrderId
	}
	if _, ok := raw["type"]; ok {
		updates["type"] = body.Type
	}
	if _, ok := raw["kepada_yth"]; ok {
		updates["kepada_yth"] = body.KepadaYth
	}
	if _, ok := raw["untuk"]; ok {
		updates["untuk"] = body.Untuk
	}
	if _, ok := raw["alamat"]; ok {
		updates["alamat"] = body.Alamat
	}
	if _, ok := raw["email"]; ok {
		updates["email"] = body.Email
	}
	if _, ok := raw["telp"]; ok {
		updates["telp"] = body.Telp
	}
	if _, ok := raw["start_produksi"]; ok {
		updates["start_produksi"] = body.StartProduksi
	}
	if _, ok := raw["lama_produksi"]; ok {
		updates["lama_produksi"] = body.LamaProduksi
	}
	if _, ok := raw["total"]; ok {
		updates["total"] = body.Total
	}
	if _, ok := raw["down_payment"]; ok {
		updates["down_payment"] = body.DownPayment
	}
	if _, ok := raw["discount"]; ok {
		updates["discount"] = body.Discount
	}
	if _, ok := raw["remaining"]; ok {
		updates["remaining"] = body.Remaining
	}
	if _, ok := raw["ar_receivable"]; ok {
		updates["ar_receivable"] = body.ArReceivable
	}
	if _, ok := raw["tanggal"]; ok {
		updates["tanggal"] = body.Tanggal
	}
	if _, ok := raw["due_date"]; ok {
		updates["due_date"] = body.DueDate
	}
	if _, ok := raw["paid_date"]; ok {
		updates["paid_date"] = body.PaidDate
	}
	if _, ok := raw["status"]; ok {
		updates["status"] = body.Status
	}

	if err := db.Model(&dto.Invoice{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var updated dto.Invoice
	db.Where("id = ?", newId).First(&updated)
	c.JSON(http.StatusOK, updated)
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
