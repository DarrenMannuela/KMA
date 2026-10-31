package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

const supplierDocumentDir = "./uploads/supplier-documents"

// Not every supplier has a clean catalogue/price-list PDF — an embroidery
// supplier, for instance, may only have photos of its materials — so a
// plain photo is accepted here too, not just a PDF.
var allowedDocumentExt = map[string]bool{
	".pdf":  true,
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

// 20MB cap — a scanned catalogue/price-list PDF or a full-res photo runs
// larger than a product photo's own 5MB cap, but still small enough that a
// handful of uploads won't fill the disk.
const maxDocumentSize = 20 << 20

// GetSupplierDocumentsBySupplier lists one supplier's catalogue/price-list
// documents, newest first.
func GetSupplierDocumentsBySupplier(c *gin.Context) {
	supplierId := c.Query("supplier_id")
	if supplierId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "supplier_id query param is required"})
		return
	}
	db := Connect()
	var docs []dto.SupplierDocument
	if err := db.Where("supplier_id = ?", supplierId).Order("uploaded_at desc").Find(&docs).Error; err != nil {
		dbError(c, err)
		return
	}
	c.JSON(http.StatusOK, docs)
}

// documentFilePath turns a stored FilePath ("/uploads/supplier-documents/5.pdf")
// into a file path, the same traversal guard as ClientItemHandler's
// photoFilePath: ok is false for anything not directly in this folder.
func documentFilePath(filePath string) (path string, ok bool) {
	name := strings.TrimPrefix(filePath, "/uploads/supplier-documents/")
	if name == filePath || name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return "", false
	}
	return filepath.Join(supplierDocumentDir, name), true
}

// UploadSupplierDocument attaches a new catalogue/price-list document (a
// PDF, or a plain photo for a supplier with nothing cleaner) to a supplier.
// Unlike a client-item photo, a supplier can have several, so
// each upload adds a new row and file rather than replacing one — the row
// is created first so the file on disk can be named by its own id, which
// can never collide between two documents of the same supplier.
func UploadSupplierDocument(c *gin.Context) {
	supplierId := c.Param("id")
	db := Connect()

	var supplier dto.Supplier
	if err := db.Where("id = ?", supplierId).First(&supplier).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided (expected form field \"file\")"})
		return
	}

	if file.Size > maxDocumentSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 20MB)"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedDocumentExt[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file type — use pdf, jpg, jpeg, or png"})
		return
	}

	if err := os.MkdirAll(supplierDocumentDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not prepare upload directory"})
		return
	}

	doc := dto.SupplierDocument{
		SupplierId: supplier.Id,
		FileName:   file.Filename,
		UploadedAt: time.Now(),
	}
	if err := db.Create(&doc).Error; err != nil {
		dbError(c, err)
		return
	}

	filename := fmt.Sprintf("%d%s", doc.Id, ext)
	fullPath := filepath.Join(supplierDocumentDir, filename)
	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		db.Delete(&doc)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	doc.FilePath = fmt.Sprintf("/uploads/supplier-documents/%s", filename)
	if err := db.Model(&doc).Update("file_path", doc.FilePath).Error; err != nil {
		dbError(c, err)
		return
	}

	c.JSON(http.StatusOK, doc)
}

// DeleteSupplierDocument removes one document — its file, then its row.
func DeleteSupplierDocument(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var doc dto.SupplierDocument
	if err := db.Where("id = ?", id).First(&doc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}

	if path, ok := documentFilePath(doc.FilePath); ok {
		os.Remove(path)
	}

	if err := db.Delete(&doc).Error; err != nil {
		dbError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
