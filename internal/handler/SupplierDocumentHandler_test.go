package handler

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
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

// The smallest PDF that starts and ends the way a real one does.
var testPDF = []byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\ntrailer << /Root 1 0 R >>\n%%EOF\n")

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for x := 0; x < 40; x++ {
		for y := 0; y < 30; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 6), 90, uint8(y * 8), 255})
		}
	}
	return img
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, testImage(), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, testImage()); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// What a file contains is checked, not just its name: a renamed file, a
// cut-off PDF and a damaged photo are each refused with a 400 and a
// message saying what to do; whole files of each kind go through.
func TestSupplierDocumentContentChecks(t *testing.T) {
	r := supplierDocumentRouter()
	sid := testSupplier(t)
	t.Cleanup(func() { os.RemoveAll(supplierDocumentDir) })
	path := "/supplier/" + sid + "/document"

	jpg := testJPEG(t)
	pngBytes := testPNG(t)
	refused := []struct {
		name, file string
		content    []byte
	}{
		{"text renamed to .pdf", "catalogue.pdf", []byte("just some notes, not a PDF at all")},
		{"photo renamed to .pdf", "catalogue.pdf", jpg},
		{"PDF renamed to .jpg", "swatches.jpg", testPDF},
		{"PNG renamed to .jpg", "swatches.jpg", pngBytes},
		{"cut-off PDF", "catalogue.pdf", testPDF[:len(testPDF)-8]},
		{"cut-off JPEG", "swatches.jpg", jpg[:len(jpg)/2]},
		{"cut-off PNG", "swatches.png", pngBytes[:len(pngBytes)/2]},
		{"empty file", "empty.pdf", []byte{}},
	}
	for _, c := range refused {
		w := uploadFile(t, r, path, "file", c.file, c.content)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400 (%s)", c.name, w.Code, w.Body.String())
			continue
		}
		var body map[string]string
		json.Unmarshal(w.Body.Bytes(), &body)
		if body["error"] == "" {
			t.Errorf("%s: no error message for the person uploading", c.name)
		}
	}

	accepted := []struct {
		file    string
		content []byte
	}{
		{"catalogue.pdf", testPDF},
		{"catalogue.PDF", testPDF},
		{"swatches.jpg", jpg},
		{"swatches.jpeg", jpg},
		{"swatches.png", pngBytes},
		// A PDF with a little trailing junk after its end marker still opens.
		{"trailing.pdf", append(append([]byte{}, testPDF...), []byte("\r\n\x00\x00")...)},
	}
	for _, c := range accepted {
		if w := uploadFile(t, r, path, "file", c.file, c.content); w.Code != http.StatusOK {
			t.Errorf("%s: %d, want 200 (%s)", c.file, w.Code, w.Body.String())
		}
	}

	// A refused upload leaves no row or file behind.
	var count int64
	Connect().Model(&dto.SupplierDocument{}).Where("supplier_id = ?", sid).Count(&count)
	if count != int64(len(accepted)) {
		t.Errorf("documents saved: %d, want %d", count, len(accepted))
	}
	Connect().Where("supplier_id = ?", sid).Delete(&dto.SupplierDocument{})
}

// A supplier's catalogue PDF (or, for a supplier with nothing cleaner, a
// plain photo) can be uploaded, listed, downloaded, and deleting one
// removes both its row and its file on disk.
func TestSupplierDocumentUpload(t *testing.T) {
	r := supplierDocumentRouter()
	sid := testSupplier(t)
	t.Cleanup(func() { os.RemoveAll(supplierDocumentDir) })

	w := uploadFile(t, r, "/supplier/"+sid+"/document", "file", "Price List 2026.pdf", testPDF)
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
	if w := uploadFile(t, r, "/supplier/"+sid+"/document", "file", "materials.jpg", testJPEG(t)); w.Code != http.StatusOK {
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
