package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetClientItems(c *gin.Context) {
	var items []dto.ClientItem
	db := Connect()

	results := db.Find(&items)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
		return
	}

	c.JSON(200, items)
}

// GetClientItemsByClient loads one client's independent catalogue — same
// shape as GetItemsByOrder in ItemHandler.go.
func GetClientItemsByClient(c *gin.Context) {
	clientId := c.Query("client_id")
	var items []dto.ClientItem
	db := Connect()

	result := db.Where("client_id = ?", clientId).Find(&items)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(200, items)
}

func GetClientItemByID(c *gin.Context) {
	id := c.Param("id")
	var item dto.ClientItem
	db := Connect()

	if err := db.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client item not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// PostClientItem intentionally does NOT upsert — unlike order line items
// (Items.go), a duplicate client_id/item_name/size here is always a
// mistake, so idx_client_items_dedupe just rejects it. The DB error
// surfaces as a 500; there's no silent merge to fall back to.
func PostClientItem(c *gin.Context) {
	var newItem dto.ClientItem
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newItem); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if result := db.Create(&newItem); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed — this client may already have an item with this name/size"})
		return
	}
	c.JSON(201, newItem)
}

func UpdateClientItem(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.ClientItem
	if err := db.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client item not found"})
		return
	}

	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	var body dto.ClientItem
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	updates := map[string]interface{}{}
	if _, ok := raw["client_id"]; ok {
		updates["client_id"] = body.ClientId
	}
	if _, ok := raw["item_name"]; ok {
		updates["item_name"] = body.ItemName
	}
	if _, ok := raw["size"]; ok {
		updates["size"] = body.Size
	}
	if _, ok := raw["notes"]; ok {
		updates["notes"] = body.Notes
	}

	if len(updates) > 0 {
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	var updated dto.ClientItem
	db.First(&updated, id)
	c.JSON(http.StatusOK, updated)
}

func DeleteClientItem(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.ClientItem
	if err := db.First(&existing, id).Error; err == nil && existing.PhotoPath != nil {
		os.Remove("." + *existing.PhotoPath)
	}

	result := db.Where("id = ?", id).Delete(&dto.ClientItem{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client item not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

const clientItemPhotoDir = "./uploads/client-items"

var allowedPhotoExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// UploadClientItemPhoto saves a product photo for one catalogue item to
// local disk and records its path on the row. Filename is keyed on the
// item's own id, so a re-upload naturally overwrites the previous photo
// instead of accumulating orphaned files, and there's never a filename
// collision to worry about since ClientItem.Id is already unique.
func UploadClientItemPhoto(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.ClientItem
	if err := db.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client item not found"})
		return
	}

	file, err := c.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No photo file provided (expected form field \"photo\")"})
		return
	}

	// 5MB cap — generous for a product photo, small enough that a
	// handful of accidental full-res camera uploads won't fill the disk.
	const maxPhotoSize = 5 << 20
	if file.Size > maxPhotoSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Photo too large (max 5MB)"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedPhotoExt[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file type — use jpg, png, or webp"})
		return
	}

	if err := os.MkdirAll(clientItemPhotoDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not prepare upload directory"})
		return
	}

	filename := fmt.Sprintf("%s%s", id, ext)
	fullPath := filepath.Join(clientItemPhotoDir, filename)

	// If the item previously had a photo under a DIFFERENT extension
	// (e.g. swapping a .png for a .jpg), remove the stale file so it
	// doesn't linger unreferenced on disk.
	if existing.PhotoPath != nil {
		oldFullPath := "." + *existing.PhotoPath
		if oldFullPath != fullPath {
			os.Remove(oldFullPath)
		}
	}

	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save photo"})
		return
	}

	photoPath := "/uploads/client-items/" + filename
	if err := db.Model(&existing).Update("photo_path", photoPath).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var updated dto.ClientItem
	db.First(&updated, id)
	c.JSON(http.StatusOK, updated)
}

// DeleteClientItemPhoto removes just the photo, leaving the catalogue
// item (and its price history) intact — e.g. the client's reference
// image is out of date but the item itself should stay.
func DeleteClientItemPhoto(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.ClientItem
	if err := db.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client item not found"})
		return
	}

	if existing.PhotoPath != nil {
		os.Remove("." + *existing.PhotoPath)
	}

	// Map-based Updates (not struct-based) so a nil value is actually
	// written as NULL rather than silently skipped as a Go zero value.
	if err := db.Model(&existing).Updates(map[string]interface{}{"photo_path": nil}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var updated dto.ClientItem
	db.First(&updated, id)
	c.JSON(http.StatusOK, updated)
}
