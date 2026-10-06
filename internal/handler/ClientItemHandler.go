package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	if err := db.Where("id = ?", id).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client item not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// PostClientItem doesn't upsert: a duplicate catalogue item is always a
// mistake, and idx_client_items_dedupe rejects it.
func PostClientItem(c *gin.Context) {
	var newItem dto.ClientItem
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newItem); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	// The photo is only ever set by UploadClientItemPhoto. A photo_path in
	// the request is ignored: it's a path on this server's disk, and
	// deleting the item (or replacing its photo) deletes that file — taken
	// from the request, it could have named any file, the database
	// included.
	newItem.PhotoPath = nil

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
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
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
	if err := db.Where("id = ?", id).First(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update saved but the record could not be reloaded"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func DeleteClientItem(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.ClientItem
	if err := db.Where("id = ?", id).First(&existing).Error; err == nil && existing.PhotoPath != nil {
		removePhotoFile(*existing.PhotoPath)
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
}

// photoFilePath turns a stored PhotoPath ("/uploads/client-items/5.png?v=…")
// into a file path, dropping the cache-busting ?v=. ok is false for anything
// not directly in the photo folder (another folder, "..", an absolute path):
// callers then leave the disk alone.
func photoFilePath(photoPath string) (path string, ok bool) {
	p := photoPath
	if u, err := url.Parse(photoPath); err == nil {
		p = u.Path
	}
	name := strings.TrimPrefix(p, "/uploads/client-items/")
	if name == p || name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return "", false
	}
	return filepath.Join(clientItemPhotoDir, name), true
}

// removePhotoFile deletes the photo a stored PhotoPath points to, if it's
// one of ours.
func removePhotoFile(photoPath string) {
	if path, ok := photoFilePath(photoPath); ok {
		os.Remove(path)
	}
}

// UploadClientItemPhoto saves an item's photo to disk, named by the item's
// id, so a new upload replaces the old file.
func UploadClientItemPhoto(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.ClientItem
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file type — use jpg, jpeg, or png"})
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
		if oldFullPath, ok := photoFilePath(*existing.PhotoPath); ok && oldFullPath != fullPath {
			os.Remove(oldFullPath)
		}
	}

	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save photo"})
		return
	}

	// ?v=<timestamp> makes browsers fetch the replaced photo.
	photoPath := fmt.Sprintf("/uploads/client-items/%s?v=%d", filename, time.Now().UnixMilli())
	if err := db.Model(&existing).Update("photo_path", photoPath).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var updated dto.ClientItem
	if err := db.Where("id = ?", id).First(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "photo saved but the record could not be reloaded"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// DeleteClientItemPhoto removes just the photo, leaving the catalogue
// item (and its price history) intact — e.g. the client's reference
// image is out of date but the item itself should stay.
func DeleteClientItemPhoto(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.ClientItem
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client item not found"})
		return
	}

	if existing.PhotoPath != nil {
		removePhotoFile(*existing.PhotoPath)
	}

	// Map-based Updates (not struct-based) so a nil value is actually
	// written as NULL rather than silently skipped as a Go zero value.
	if err := db.Model(&existing).Updates(map[string]interface{}{"photo_path": nil}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var updated dto.ClientItem
	if err := db.Where("id = ?", id).First(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "photo removed but the record could not be reloaded"})
		return
	}
	c.JSON(http.StatusOK, updated)
}
