package handler

import (
	"errors"
	"gorm.io/gorm"
	"net/http"
	"strconv"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetBuktiKasItems(c *gin.Context) {
	var items []dto.BuktiKasItem
	db := Connect()

	if err := db.Find(&items).Error; err != nil {
		dbError(c, err)
		return
	}

	c.JSON(http.StatusOK, items)
}

func GetBuktiKasItemsByHeader(c *gin.Context) {
	headerId := c.Query("header_id")
	if headerId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "header_id is required"})
		return
	}

	var items []dto.BuktiKasItem
	db := Connect()

	if err := db.Where("header_id = ?", headerId).Find(&items).Error; err != nil {
		dbError(c, err)
		return
	}

	c.JSON(http.StatusOK, items)
}

func GetBuktiKasItemsGrouped(c *gin.Context) {
	var items []dto.BuktiKasItem
	db := Connect()

	if err := db.Find(&items).Error; err != nil {
		dbError(c, err)
		return
	}

	grouped := make(map[string][]dto.BuktiKasItem)
	for _, item := range items {
		grouped[item.HeaderId] = append(grouped[item.HeaderId], item)
	}

	c.JSON(http.StatusOK, grouped)
}

func PostBuktiKasItem(c *gin.Context) {
	var item dto.BuktiKasItem

	db := Connect()
	if err := c.ShouldBindBodyWithJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if item.HeaderId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "header_id is required"})
		return
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		if item.InvoiceId != nil && *item.InvoiceId != "" {
			order, err := checkInvoiceLink(tx, *item.InvoiceId, item.OrderId, 0)
			if err != nil {
				return err
			}
			item.OrderId = &order
		} else {
			item.InvoiceId = nil
		}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		invoices := map[string]bool{}
		linkedInvoices([]dto.BuktiKasItem{item}, invoices)
		return reconcileInvoicePayments(tx, invoices)
	})
	if respondBuktiKasError(c, err, "Database insert failed") {
		return
	}

	c.JSON(http.StatusCreated, item)
}

func UpdateBuktiKasItem(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item id"})
		return
	}

	db := Connect()
	var existing dto.BuktiKasItem
	if err := db.First(&existing, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bukti Kas item not found"})
		return
	}
	originalId := existing.Id

	if err := c.ShouldBindBodyWithJSON(&existing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	// Pin the PK back after binding — a client-sent "id" would otherwise
	// redirect Save()'s WHERE clause and silently no-op instead of
	// updating this row.
	existing.Id = originalId

	var before dto.BuktiKasItem
	db.First(&before, "id = ?", id)
	err = db.Transaction(func(tx *gorm.DB) error {
		if existing.InvoiceId != nil && *existing.InvoiceId == "" {
			existing.InvoiceId = nil
		}
		if existing.InvoiceId != nil {
			order, err := checkInvoiceLink(tx, *existing.InvoiceId, existing.OrderId, existing.Id)
			if err != nil {
				return err
			}
			existing.OrderId = &order
		}
		if err := tx.Save(&existing).Error; err != nil {
			return err
		}
		invoices := map[string]bool{}
		linkedInvoices([]dto.BuktiKasItem{before, existing}, invoices)
		return reconcileInvoicePayments(tx, invoices)
	})
	if respondBuktiKasError(c, err, "Update failed") {
		return
	}

	c.JSON(http.StatusOK, existing)
}

func DeleteBuktiKasItem(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item id"})
		return
	}

	db := Connect()
	var result *gorm.DB
	err = db.Transaction(func(tx *gorm.DB) error {
		var gone []dto.BuktiKasItem
		if err := tx.Where("id = ?", id).Find(&gone).Error; err != nil {
			return err
		}
		result = tx.Where("id = ?", id).Delete(&dto.BuktiKasItem{})
		if result.Error != nil {
			return result.Error
		}
		invoices := map[string]bool{}
		linkedInvoices(gone, invoices)
		return reconcileInvoicePayments(tx, invoices)
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bukti Kas item not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// respondBuktiKasError answers a failed write: a link that isn't allowed
// with its own status and message, anything else as fallback. It reports
// whether it answered.
func respondBuktiKasError(c *gin.Context, err error, fallback string) bool {
	if err == nil {
		return false
	}
	var be batchError
	if errors.As(err, &be) {
		c.JSON(be.status, gin.H{"error": be.msg})
		return true
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": fallback})
	return true
}
