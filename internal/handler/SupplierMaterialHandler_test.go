package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func supplierMaterialRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/supplier-material", GetSupplierMaterials)
	r.GET("/supplier-material/by-supplier", GetSupplierMaterialsBySupplier)
	r.GET("/supplier-material/:id", GetSupplierMaterialByID)
	r.POST("/supplier-material", PostSupplierMaterial)
	r.PATCH("/supplier-material/:id", UpdateSupplierMaterial)
	r.DELETE("/supplier-material/:id", DeleteSupplierMaterial)
	r.GET("/supplier-material-color", GetSupplierMaterialColors)
	r.GET("/supplier-material-color/by-material", GetSupplierMaterialColorsByMaterial)
	r.GET("/supplier-material-color/grouped", GetSupplierMaterialColorsGrouped)
	r.POST("/supplier-material-color", PostSupplierMaterialColor)
	r.PATCH("/supplier-material-color/:id", UpdateSupplierMaterialColor)
	r.DELETE("/supplier-material-color/:id", DeleteSupplierMaterialColor)
	return r
}

// A material can be added to a supplier's catalogue, given a colour, found
// by the grouped lookup the picker uses, edited, and removing the material
// takes its colour with it.
func TestSupplierMaterialCatalogue(t *testing.T) {
	r := supplierMaterialRouter()
	sid := testSupplier(t)

	w := callOn(t, r, http.MethodPost, "/supplier-material", `{"supplier_id": `+sid+`, "name": "TEST DRILL", "brand": "ACME", "price": 50000}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create material: %d %s", w.Code, w.Body.String())
	}
	var mat dto.SupplierMaterial
	json.Unmarshal(w.Body.Bytes(), &mat)
	if mat.Price != 50000 || mat.Brand == nil || *mat.Brand != "ACME" {
		t.Fatalf("created material: %+v", mat)
	}
	matId := itoa(uint(mat.Id))

	// Duplicate (supplier_id, name, brand) is rejected.
	if w := callOn(t, r, http.MethodPost, "/supplier-material", `{"supplier_id": `+sid+`, "name": "TEST DRILL", "brand": "ACME"}`); w.Code != http.StatusInternalServerError {
		t.Errorf("duplicate material: %d, want 500", w.Code)
	}

	// The same name under a DIFFERENT brand is a different material, not a
	// duplicate — SAI's real catalogue has exactly this shape ("Tropical
	// Deluxe" from both Verlando and Maryland, "USA Drill" from both
	// Ventura and Venhouston).
	if w := callOn(t, r, http.MethodPost, "/supplier-material", `{"supplier_id": `+sid+`, "name": "TEST DRILL", "brand": "OTHER BRAND"}`); w.Code != http.StatusCreated {
		t.Errorf("same name, different brand: %d, want 201", w.Code)
	}

	// A material needs a real supplier.
	if w := callOn(t, r, http.MethodPost, "/supplier-material", `{"supplier_id": 999999, "name": "NOPE"}`); w.Code != http.StatusBadRequest {
		t.Errorf("bogus supplier: %d, want 400", w.Code)
	}

	w = callOn(t, r, http.MethodPost, "/supplier-material-color", `{"supplier_material_id": `+matId+`, "code": "01", "category": "Navy", "hex": "#1e293b"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create colour: %d %s", w.Code, w.Body.String())
	}
	var col dto.SupplierMaterialColor
	json.Unmarshal(w.Body.Bytes(), &col)

	// Duplicate (material, code) is rejected.
	if w := callOn(t, r, http.MethodPost, "/supplier-material-color", `{"supplier_material_id": `+matId+`, "code": "01", "hex": "#000"}`); w.Code != http.StatusInternalServerError {
		t.Errorf("duplicate colour: %d, want 500", w.Code)
	}

	// The grouped endpoint — what the frontend's live picker loads.
	w = callOn(t, r, http.MethodGet, "/supplier-material-color/grouped", "")
	var grouped map[string][]dto.SupplierMaterialColor
	json.Unmarshal(w.Body.Bytes(), &grouped)
	if cs := grouped[matId]; len(cs) != 1 || cs[0].Code != "01" {
		t.Errorf("grouped colours for %s: %+v", matId, grouped)
	}

	// by-supplier finds the material.
	w = callOn(t, r, http.MethodGet, "/supplier-material/by-supplier?supplier_id="+sid, "")
	var bySupplier []dto.SupplierMaterial
	json.Unmarshal(w.Body.Bytes(), &bySupplier)
	found := false
	for _, m := range bySupplier {
		if m.Id == mat.Id {
			found = true
		}
	}
	if !found {
		t.Errorf("material missing from by-supplier: %+v", bySupplier)
	}

	// A price edit updates just that field.
	w = callOn(t, r, http.MethodPatch, "/supplier-material/"+matId, `{"price": 55000}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update price: %d %s", w.Code, w.Body.String())
	}
	var updated dto.SupplierMaterial
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Price != 55000 || updated.Name != "TEST DRILL" {
		t.Errorf("after price update: %+v", updated)
	}

	// Deleting the material cascades to its colour.
	if w := callOn(t, r, http.MethodDelete, "/supplier-material/"+matId, ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete material: %d %s", w.Code, w.Body.String())
	}
	var goneCol dto.SupplierMaterialColor
	if Connect().Where("id = ?", col.Id).First(&goneCol).Error == nil {
		t.Error("colour survived its material's deletion")
	}
}
