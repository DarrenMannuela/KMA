package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func supplierDesignRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/supplier-design", GetSupplierDesigns)
	r.GET("/supplier-design/by-supplier", GetSupplierDesignsBySupplier)
	r.POST("/supplier-design", PostSupplierDesign)
	r.PATCH("/supplier-design/:id", UpdateSupplierDesign)
	r.DELETE("/supplier-design/:id", DeleteSupplierDesign)
	r.POST("/supplier-design/:id/photo", UploadSupplierDesignPhoto)
	r.DELETE("/supplier-design/:id/photo", DeleteSupplierDesignPhoto)
	return r
}

// A bordir/sablon design: created, listed, refused when it repeats the same
// name, placement and size, edited (null clears a field), given a real photo
// (a fake one is refused), and deleted along with its photo file.
func TestSupplierDesigns(t *testing.T) {
	r := supplierDesignRouter()
	sid := testSupplier(t)
	t.Cleanup(func() {
		Connect().Where("supplier_id = ?", sid).Delete(&dto.SupplierDesign{})
		os.RemoveAll(supplierDesignDir)
	})

	w := callOn(t, r, http.MethodPost, "/supplier-design", `{"supplier_id": `+sid+`, "name": "LOGO PT CONTOH", "placement": "DADA KIRI", "size": "7 CM", "price": 7500}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var d dto.SupplierDesign
	json.Unmarshal(w.Body.Bytes(), &d)
	id := itoa(uint(d.Id))

	// The same logo at another spot is another design; the same spot again isn't.
	if w := callOn(t, r, http.MethodPost, "/supplier-design", `{"supplier_id": `+sid+`, "name": "LOGO PT CONTOH", "placement": "PUNGGUNG", "size": "25 CM", "price": 22000}`); w.Code != http.StatusCreated {
		t.Errorf("same logo, other placement: %d", w.Code)
	}
	if w := callOn(t, r, http.MethodPost, "/supplier-design", `{"supplier_id": `+sid+`, "name": "LOGO PT CONTOH", "placement": "DADA KIRI", "size": "7 CM", "price": 1}`); w.Code != http.StatusConflict {
		t.Errorf("duplicate design: %d, want 409", w.Code)
	}
	if w := callOn(t, r, http.MethodPost, "/supplier-design", `{"supplier_id": `+sid+`, "name": "  "}`); w.Code != http.StatusBadRequest {
		t.Errorf("blank name: %d, want 400", w.Code)
	}

	w = callOn(t, r, http.MethodGet, "/supplier-design/by-supplier?supplier_id="+sid, "")
	var list []dto.SupplierDesign
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatalf("by-supplier: %d designs, want 2", len(list))
	}

	if w := callOn(t, r, http.MethodPatch, "/supplier-design/"+id, `{"price": 8000, "size": null}`); w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	var got dto.SupplierDesign
	Connect().First(&got, d.Id)
	if got.Price != 8000 || got.Size != nil || got.Placement == nil || *got.Placement != "DADA KIRI" {
		t.Errorf("after update: %+v", got)
	}

	// Photos: a fake is refused, a real one is kept and served from its own folder.
	if w := uploadFile(t, r, "/supplier-design/"+id+"/photo", "photo", "logo.jpg", []byte("not a photo")); w.Code != http.StatusBadRequest {
		t.Errorf("fake photo: %d, want 400", w.Code)
	}
	w = uploadFile(t, r, "/supplier-design/"+id+"/photo", "photo", "logo.png", testPNG(t))
	if w.Code != http.StatusOK {
		t.Fatalf("photo: %d %s", w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.PhotoPath == nil || !strings.HasPrefix(*got.PhotoPath, "/uploads/supplier-designs/"+id+".png") {
		t.Fatalf("photo path: %v", got.PhotoPath)
	}
	file, _ := designPhotoFile(*got.PhotoPath)
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("photo not on disk: %v", err)
	}

	// Deleting the design takes its photo with it.
	if w := callOn(t, r, http.MethodDelete, "/supplier-design/"+id, ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("photo still on disk after delete")
	}
	if w := callOn(t, r, http.MethodDelete, "/supplier-design/"+id, ""); w.Code != http.StatusNotFound {
		t.Errorf("second delete: %d, want 404", w.Code)
	}

	// A photo path that tries to leave the folder is never treated as a file.
	if _, ok := designPhotoFile("/uploads/supplier-designs/../../db.sqlite"); ok {
		t.Error("path traversal accepted")
	}
}
