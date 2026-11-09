package handler

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
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

// What each allowed ending must really contain, by the file's first bytes
// (http.DetectContentType), not by its name: a renamed file that isn't
// really a PDF or photo is refused.
var documentContentType = map[string]string{
	".pdf":  "application/pdf",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
}

// A photo bigger than this many pixels is refused before it's decoded: a
// phone photo is ~12-50 million; this only stops a file built to eat memory.
const maxDocumentPixels = 100_000_000

// checkDocumentFile opens the upload and makes sure it is what its name
// says and that it opens: a photo must decode completely (a cut-off or
// damaged one doesn't), and a PDF must start like one and end with its
// end-of-file marker (a cut-off download doesn't). The error is shown to
// the person uploading, so it says what to do.
func checkDocumentFile(fh *multipart.FileHeader, ext string) error {
	f, err := fh.Open()
	if err != nil {
		return errors.New("Couldn't read the file. Try uploading it again.")
	}
	defer f.Close()

	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	if n == 0 {
		return errors.New("The file is empty.")
	}
	want := documentContentType[ext]
	if got := http.DetectContentType(head); !strings.HasPrefix(got, want) {
		kind := strings.ToUpper(strings.TrimPrefix(ext, "."))
		return fmt.Errorf("This file is named .%s but isn't really a %s. Export or save it again as a real %s and upload that.", strings.TrimPrefix(ext, "."), kind, kind)
	}

	if want == "application/pdf" {
		// The end-of-file marker sits in the last few bytes; allow some
		// trailing whitespace or junk after it, as real PDFs often have.
		size := fh.Size
		tailLen := int64(2048)
		if size < tailLen {
			tailLen = size
		}
		tail := make([]byte, tailLen)
		if _, err := f.ReadAt(tail, size-tailLen); err != nil && err != io.EOF {
			return errors.New("Couldn't read the file. Try uploading it again.")
		}
		if !bytes.Contains(tail, []byte("%%EOF")) {
			return errors.New("This PDF looks incomplete (it may have been cut off while downloading). Download it again and upload the new copy.")
		}
		return nil
	}

	// A photo: its size first (cheap), then a full decode, which is what
	// catches a cut-off or damaged image.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return errors.New("Couldn't read the file. Try uploading it again.")
	}
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return errors.New("This photo is damaged and can't be opened. Take or export it again.")
	}
	if cfg.Width*cfg.Height > maxDocumentPixels {
		return errors.New("This photo is too large to open (over 100 megapixels). Save a smaller copy and upload that.")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return errors.New("Couldn't read the file. Try uploading it again.")
	}
	if _, _, err := image.Decode(f); err != nil {
		return errors.New("This photo is incomplete or damaged and can't be opened fully. Take or export it again.")
	}
	return nil
}

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

	if err := checkDocumentFile(file, ext); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
