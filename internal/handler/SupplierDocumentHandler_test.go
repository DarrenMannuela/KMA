package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func supplierDocumentRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/supplier-document/by-supplier", GetSupplierDocumentsBySupplier)
	r.POST("/supplier/:id/document", UploadSupplierDocument)
	r.DELETE("/supplier-document/:id", DeleteSupplierDocument)
	return r
}

// uploadFile posts a tiny in-memory file as the given form field.
func uploadFile(t *testing.T, r *gin.Engine, path, field, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(content)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// A supplier's catalogue PDF (or, for a supplier with nothing cleaner, a
// plain photo) can be uploaded, listed, downloaded, and deleting one
// removes both its row and its file on disk.
func TestSupplierDocumentUpload(t *testing.T) {
	r := supplierDocumentRouter()
	sid := testSupplier(t)
	t.Cleanup(func() { os.RemoveAll(supplierDocumentDir) })

	w := uploadFile(t, r, "/supplier/"+sid+"/document", "file", "Price List 2026.pdf", []byte("%PDF-1.4 fake"))
	if w.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	var doc dto.SupplierDocument
	json.Unmarshal(w.Body.Bytes(), &doc)
	if doc.FileName != "Price List 2026.pdf" || doc.FilePath == "" {
		t.Fatalf("uploaded doc: %+v", doc)
	}
	if _, err := os.Stat(supplierDocumentDir + "/" + itoa(uint(doc.Id)) + ".pdf"); err != nil {
		t.Fatalf("file not saved to disk: %v", err)
	}

	// A plain photo is accepted too — not every supplier has a clean PDF.
	if w := uploadFile(t, r, "/supplier/"+sid+"/document", "file", "materials.jpg", []byte("fake jpeg")); w.Code != http.StatusOK {
		t.Errorf("photo upload: %d, want 200", w.Code)
	}

	// An arbitrary file type is still rejected.
	if w := uploadFile(t, r, "/supplier/"+sid+"/document", "file", "notes.txt", []byte("hi")); w.Code != http.StatusBadRequest {
		t.Errorf("unsupported file type: %d, want 400", w.Code)
	}

	// A document needs a real supplier.
	if w := uploadFile(t, r, "/supplier/999999/document", "file", "x.pdf", []byte("%PDF")); w.Code != http.StatusNotFound {
		t.Errorf("bogus supplier: %d, want 404", w.Code)
	}

	// by-supplier finds both.
	w = callOn(t, r, http.MethodGet, "/supplier-document/by-supplier?supplier_id="+sid, "")
	var docs []dto.SupplierDocument
	json.Unmarshal(w.Body.Bytes(), &docs)
	if len(docs) != 2 {
		t.Errorf("by-supplier: %+v", docs)
	}

	docId := itoa(uint(doc.Id))
	if w := callOn(t, r, http.MethodDelete, "/supplier-document/"+docId, ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(supplierDocumentDir + "/" + docId + ".pdf"); err == nil {
		t.Error("file survived its document's deletion")
	}
	var gone dto.SupplierDocument
	if Connect().Where("id = ?", doc.Id).First(&gone).Error == nil {
		t.Error("row survived deletion")
	}
}
