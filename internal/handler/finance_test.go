package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func financeRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/finance/batch", PostFinanceBatch)
	r.GET("/budget", GetBudgets)
	r.PUT("/budget", PutBudget)
	r.POST("/recurring-cost", PostRecurringCost)
	r.POST("/recurring-cost/post-month", PostRecurringMonth)
	r.POST("/order", PostOrders)
	r.PATCH("/order/*id", UpdateOrders)
	r.DELETE("/order/*id", DeleteOrders)
	return r
}

func batch(t *testing.T, body string, want int) FinanceBatchResult {
	t.Helper()
	w := callOn(t, financeRouter(), http.MethodPost, "/finance/batch", body)
	if w.Code != want {
		t.Fatalf("batch: status %d, want %d: %s", w.Code, want, w.Body.String())
	}
	var res FinanceBatchResult
	json.Unmarshal(w.Body.Bytes(), &res)
	return res
}

// testSupplier is a supplier the production lines in these tests can use.
func testSupplier(t *testing.T) string {
	t.Helper()
	var s dto.Supplier
	if Connect().Where("supplier_name = ?", "Test Fabrics").First(&s).Error != nil {
		s = dto.Supplier{SupplierName: "Test Fabrics", SupplierCategory: "general_supplier"}
		if err := Connect().Create(&s).Error; err != nil {
			t.Fatal(err)
		}
	}
	return strconv.FormatUint(uint64(s.Id), 10)
}

func countLines(header string) (prod, op int64) {
	Connect().Model(&dto.ProductionItem{}).Where("header_id = ?", header).Count(&prod)
	Connect().Model(&dto.OperationItem{}).Where("header_id = ?", header).Count(&op)
	return
}

func headerExists(id string) bool {
	var n int64
	Connect().Model(&dto.FinanceHeader{}).Where("id = ?", id).Count(&n)
	return n > 0
}

// A new Kas Bon with production and operation lines is saved in one go, and a
// quantity of 2.5 meters is kept as 2.5.
func TestBatchSavesAWholeKasBon(t *testing.T) {
	sup := testSupplier(t)
	res := batch(t, `{
		"new_headers": [{"id": "01/KB/90", "date": "2090-01-05", "description": "BELI BAHAN"}],
		"production_create": [
			{"header_id": "01/KB/90", "material_name": "DRILL", "price": 30000, "si_unit": "meter", "amount": 2.5, "supplier_id": `+sup+`},
			{"header_id": "01/KB/90", "material_name": "KANCING", "price": 500, "si_unit": "pcs", "amount": 100, "supplier_id": `+sup+`}
		],
		"operation_create": [{"header_id": "01/KB/90", "category": "TRANSPORT", "description": "OJEK", "price": 20000}]
	}`, http.StatusOK)
	if len(res.HeadersCreated) != 1 || len(res.Production) != 2 || len(res.Operation) != 1 {
		t.Fatalf("created %+v", res)
	}
	var drill dto.ProductionItem
	Connect().Where("material_name = ?", "DRILL").First(&drill)
	if drill.Amount != 2.5 {
		t.Errorf("amount %v, want 2.5", drill.Amount)
	}

	// The same ID again as a new Kas Bon is refused, and nothing in that
	// batch is saved.
	batch(t, `{
		"new_headers": [{"id": "01/KB/90", "date": "2090-01-06"}],
		"production_create": [{"header_id": "01/KB/90", "material_name": "EXTRA", "price": 1, "amount": 1, "supplier_id": `+sup+`}]
	}`, http.StatusConflict)
	batch(t, `{"headers": [{"id": "01/KB/90"}], "production_create": [{"header_id": "01/KB/90", "material_name": "X", "price": 1, "amount": 1}]}`, http.StatusBadRequest)
	if p, o := countLines("01/KB/90"); p != 2 || o != 1 {
		t.Errorf("after the refused batch: %d production, %d operation lines", p, o)
	}
}

// Moving every line off a Kas Bon removes it; deleting lines hands back what
// was deleted, and sending that back restores it exactly.
func TestBatchBulkEditAndUndo(t *testing.T) {
	res := batch(t, `{
		"new_headers": [{"id": "02/KB/90", "date": "2090-02-01", "description": "A"}, {"id": "03/KB/90", "date": "2090-02-02", "description": "B"}],
		"operation_create": [
			{"header_id": "02/KB/90", "category": "LISTRIK", "description": "PLN", "price": 100000},
			{"header_id": "03/KB/90", "category": "AIR", "description": "PDAM", "price": 50000}
		]
	}`, http.StatusOK)
	moved := res.Operation[0].Id
	out := batch(t, `{"operation_update": [{"id": `+itoa(moved)+`, "fields": {"header_id": "03/KB/90", "category": "UTILITIES"}}]}`, http.StatusOK)
	if len(out.OperationBefore) != 1 || out.OperationBefore[0].HeaderId != "02/KB/90" || out.OperationBefore[0].Category != "LISTRIK" {
		t.Errorf("before: %+v", out.OperationBefore)
	}
	if headerExists("02/KB/90") || len(out.HeadersDeleted) != 1 {
		t.Errorf("the emptied Kas Bon should be removed: %+v", out.HeadersDeleted)
	}

	del := batch(t, `{"operation_delete": [`+itoa(res.Operation[0].Id)+`, `+itoa(res.Operation[1].Id)+`]}`, http.StatusOK)
	if len(del.OperationDeleted) != 2 || len(del.HeadersDeleted) != 1 || headerExists("03/KB/90") {
		t.Fatalf("delete: %+v", del)
	}
	undo, _ := json.Marshal(FinanceBatch{Headers: del.HeadersDeleted, OperationCreate: del.OperationDeleted})
	batch(t, string(undo), http.StatusOK)
	var back []dto.OperationItem
	Connect().Where("header_id = ?", "03/KB/90").Order("id").Find(&back)
	if len(back) != 2 || back[0].Id != res.Operation[0].Id || back[0].Category != "UTILITIES" {
		t.Errorf("undo gave %+v", back)
	}

	hu := batch(t, `{"header_update": [{"id": "03/KB/90", "date": "2090-02-09", "description": "C"}]}`, http.StatusOK)
	if len(hu.HeadersBefore) != 1 || hu.HeadersBefore[0].Date != "2090-02-02" || hu.HeadersBefore[0].Description != "B" {
		t.Errorf("header before: %+v", hu.HeadersBefore)
	}
	batch(t, `{"operation_update": [{"id": `+itoa(moved)+`, "fields": {"id": 99}}]}`, http.StatusBadRequest)
	batch(t, `{"operation_update": [{"id": `+itoa(moved)+`, "fields": {"header_id": "NOPE"}}]}`, http.StatusBadRequest)
	batch(t, `{}`, http.StatusBadRequest)
}

// Costs linked to an order follow it when it's renamed and come unlinked
// when it's deleted.
func TestCostsFollowTheirOrder(t *testing.T) {
	r := financeRouter()
	if w := callOn(t, r, http.MethodPost, "/order", `{"id": "900/KMA/90", "date": "2090-03-01T00:00:00Z"}`); w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("order: %d %s", w.Code, w.Body.String())
	}
	res := batch(t, `{
		"new_headers": [{"id": "04/KB/90", "date": "2090-03-02"}],
		"production_create": [{"header_id": "04/KB/90", "material_name": "DRILL", "price": 1000, "amount": 3, "order_id": "900/KMA/90", "supplier_id": `+testSupplier(t)+`}]
	}`, http.StatusOK)
	id := res.Production[0].Id
	if w := callOn(t, r, http.MethodPatch, "/order/900%2FKMA%2F90", `{"id": "901/KMA/90"}`); w.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", w.Code, w.Body.String())
	}
	var item dto.ProductionItem
	Connect().Where("id = ?", id).First(&item)
	if item.OrderId == nil || *item.OrderId != "901/KMA/90" {
		t.Errorf("after rename the cost points at %v", item.OrderId)
	}
	if w := callOn(t, r, http.MethodDelete, "/order/901%2FKMA%2F90", ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	Connect().Where("id = ?", id).First(&item)
	if item.OrderId != nil {
		t.Errorf("after delete the cost still points at %v", *item.OrderId)
	}
}

func TestRecurringCostsPostOncePerMonth(t *testing.T) {
	r := financeRouter()
	for _, body := range []string{
		`{"category": "SEWA", "description": "SEWA RUKO", "price": 5000000, "active": true}`,
		`{"category": "INTERNET", "price": 400000, "active": true}`,
		`{"category": "LAMA", "price": 1, "active": false}`,
	} {
		if w := callOn(t, r, http.MethodPost, "/recurring-cost", body); w.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", w.Code, w.Body.String())
		}
	}
	callOn(t, r, http.MethodPost, "/recurring-cost", `{"category": "", "price": 5}`)
	w := callOn(t, r, http.MethodPost, "/recurring-cost/post-month", `{"month": "2090-05", "header_id": "05/KB/90", "date": "2090-05-01"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("post: %d %s", w.Code, w.Body.String())
	}
	if _, op := countLines("05/KB/90"); op != 2 {
		t.Errorf("posted %d lines, want the 2 active costs", op)
	}
	w = callOn(t, r, http.MethodPost, "/recurring-cost/post-month", `{"month": "2090-05", "header_id": "06/KB/90", "date": "2090-05-02"}`)
	if w.Code != http.StatusOK || headerExists("06/KB/90") {
		t.Errorf("posting the month again added a Kas Bon: %d %s", w.Code, w.Body.String())
	}
	if w := callOn(t, r, http.MethodPost, "/recurring-cost/post-month", `{"month": "2090-06", "header_id": "07/KB/90", "date": "2090-07-01"}`); w.Code != http.StatusBadRequest {
		t.Errorf("a date outside the month should be refused, got %d", w.Code)
	}
}

func TestBudgetsUpsert(t *testing.T) {
	r := financeRouter()
	callOn(t, r, http.MethodPut, "/budget", `{"scope": "operation", "category": "TRANSPORT", "amount": 1000000}`)
	callOn(t, r, http.MethodPut, "/budget", `{"scope": "operation", "category": "TRANSPORT", "amount": 1500000}`)
	callOn(t, r, http.MethodPut, "/budget", `{"scope": "production", "category": "", "amount": 20000000}`)
	if w := callOn(t, r, http.MethodPut, "/budget", `{"scope": "other", "amount": 1}`); w.Code != http.StatusBadRequest {
		t.Errorf("unknown scope: %d", w.Code)
	}
	var budgets []dto.Budget
	json.Unmarshal(callOn(t, r, http.MethodGet, "/budget", "").Body.Bytes(), &budgets)
	if len(budgets) != 2 || budgets[0].Scope != "operation" || budgets[0].Amount != 1500000 {
		t.Errorf("budgets %+v", budgets)
	}
	callOn(t, r, http.MethodPut, "/budget", `{"scope": "operation", "category": "TRANSPORT", "amount": 0}`)
	json.Unmarshal(callOn(t, r, http.MethodGet, "/budget", "").Body.Bytes(), &budgets)
	if len(budgets) != 1 {
		t.Errorf("a 0 budget should remove it: %+v", budgets)
	}
}

func callOn(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func itoa(id uint) string { return strconv.FormatUint(uint64(id), 10) }
