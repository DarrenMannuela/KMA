package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

const supplierDesignDir = "./uploads/supplier-designs"

// 10MB: a phone photo of an embroidered or printed sample.
const maxDesignPhotoSize = 10 << 20

func GetSupplierDesigns(c *gin.Context) {
	var items []dto.SupplierDesign
	if err := Connect().Order("supplier_id, name").Find(&items).Error; err != nil {
		dbError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func GetSupplierDesignsBySupplier(c *gin.Context) {
	supplierId := c.Query("supplier_id")
	if supplierId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "supplier_id query param is required"})
		return
	}
	var items []dto.SupplierDesign
	if err := Connect().Where("supplier_id = ?", supplierId).Order("name").Find(&items).Error; err != nil {
		dbError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

// PostSupplierDesign doesn't upsert: the same name, placement and size twice
// for one supplier is a mistake, and idx_supplier_designs_dedupe refuses it.
func PostSupplierDesign(c *gin.Context) {
	var item dto.SupplierDesign
	db := Connect()
	if err := c.ShouldBindBodyWithJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	item.Name = strings.TrimSpace(item.Name)
	if item.SupplierId == 0 || item.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "supplier_id and name are required"})
		return
	}
	if err := db.Where("id = ?", item.SupplierId).First(&dto.Supplier{}).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No supplier with that id"})
		return
	}
	// The photo goes up separately (POST /supplier-design/:id/photo).
	item.PhotoPath = nil
	if err := db.Create(&item).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "This supplier already has a design with this name, placement and size"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func UpdateSupplierDesign(c *gin.Context) {
	id := c.Param("id")
	db := Connect()
	var existing dto.SupplierDesign
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Design not found"})
		return
	}
	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	var body dto.SupplierDesign
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	// Map-based, so an explicit null clears a field and a missing key leaves
	// it alone. The photo has its own endpoints; supplier_id doesn't move.
	updates := map[string]interface{}{}
	if _, ok := raw["name"]; ok {
		if strings.TrimSpace(body.Name) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "A design needs a name"})
			return
		}
		updates["name"] = strings.TrimSpace(body.Name)
	}
	if _, ok := raw["placement"]; ok {
		updates["placement"] = body.Placement
	}
	if _, ok := raw["size"]; ok {
		updates["size"] = body.Size
	}
	if _, ok := raw["price"]; ok {
		updates["price"] = body.Price
	}
	if _, ok := raw["notes"]; ok {
		updates["notes"] = body.Notes
	}
	if len(updates) > 0 {
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "This supplier already has a design with this name, placement and size"})
			return
		}
	}
	var updated dto.SupplierDesign
	db.Where("id = ?", id).First(&updated)
	c.JSON(http.StatusOK, updated)
}

// designPhotoFile turns a stored PhotoPath ("/uploads/supplier-designs/5.jpg?v=…")
// into a file path; ok is false for anything not directly in that folder.
func designPhotoFile(photoPath string) (string, bool) {
	name := strings.TrimPrefix(photoPath, "/uploads/supplier-designs/")
	if i := strings.Index(name, "?"); i >= 0 {
		name = name[:i]
	}
	if name == photoPath || name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return "", false
	}
	return filepath.Join(supplierDesignDir, name), true
}

// UploadSupplierDesignPhoto sets (or replaces) a design's photo. The file is
// checked by what it contains, like a catalogue upload: a real JPG or PNG
// that opens fully.
func UploadSupplierDesignPhoto(c *gin.Context) {
	id := c.Param("id")
	db := Connect()
	var existing dto.SupplierDesign
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Design not found"})
		return
	}
	file, err := c.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No photo provided (expected form field \"photo\")"})
		return
	}
	if file.Size > maxDesignPhotoSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Photo too large (max 10MB)"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Use a JPG or PNG photo"})
		return
	}
	if err := checkDocumentFile(file, ext); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := os.MkdirAll(supplierDesignDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not prepare upload directory"})
		return
	}
	filename := fmt.Sprintf("%d%s", existing.Id, ext)
	fullPath := filepath.Join(supplierDesignDir, filename)
	if existing.PhotoPath != nil {
		if old, ok := designPhotoFile(*existing.PhotoPath); ok && old != fullPath {
			os.Remove(old)
		}
	}
	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save photo"})
		return
	}
	// ?v= so browsers fetch a replaced photo instead of the cached one.
	photoPath := fmt.Sprintf("/uploads/supplier-designs/%s?v=%d", filename, time.Now().UnixMilli())
	if err := db.Model(&existing).Update("photo_path", photoPath).Error; err != nil {
		dbError(c, err)
		return
	}
	existing.PhotoPath = &photoPath
	c.JSON(http.StatusOK, existing)
}

func DeleteSupplierDesignPhoto(c *gin.Context) {
	id := c.Param("id")
	db := Connect()
	var existing dto.SupplierDesign
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Design not found"})
		return
	}
	if existing.PhotoPath != nil {
		if path, ok := designPhotoFile(*existing.PhotoPath); ok {
			os.Remove(path)
		}
	}
	if err := db.Model(&existing).Update("photo_path", nil).Error; err != nil {
		dbError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DeleteSupplierDesign removes a design and its photo.
func DeleteSupplierDesign(c *gin.Context) {
	id := c.Param("id")
	db := Connect()
	var existing dto.SupplierDesign
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Design not found"})
		return
	}
	if existing.PhotoPath != nil {
		if path, ok := designPhotoFile(*existing.PhotoPath); ok {
			os.Remove(path)
		}
	}
	if err := db.Delete(&existing).Error; err != nil {
		dbError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
