package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func catalogueRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/client-catalogue/batch", PostCatalogueBatch)
	return r
}

func catalogue(t *testing.T, body string, want int) CatalogueBatchResult {
	t.Helper()
	w := callOn(t, catalogueRouter(), http.MethodPost, "/client-catalogue/batch", body)
	if w.Code != want {
		t.Fatalf("catalogue batch: %d, want %d: %s", w.Code, want, w.Body.String())
	}
	var res CatalogueBatchResult
	json.Unmarshal(w.Body.Bytes(), &res)
	return res
}

func priceOf(item uint64, year int) (int64, bool) {
	var p dto.ClientItemPrice
	if Connect().Where("client_item_id = ? AND year = ?", item, year).First(&p).Error != nil {
		return 0, false
	}
	return p.Price, true
}

// An item added with its sizes and prices; adding it again finds it rather
// than duplicating; a price rise reports what it changed so it can be undone.
func TestCatalogueBatch(t *testing.T) {
	cl := dto.Client{ClientName: "CATALOGUE TEST"}
	Connect().Create(&cl)
	other := dto.Client{ClientName: "OTHER CLIENT"}
	Connect().Create(&other)
	cid := itoa(uint(cl.Id))

	res := catalogue(t, `{"client_id": `+cid+`, "upsert_items": [
		{"item_name": "kemeja pdh", "size": "m", "prices": [{"year": 2026, "price": 140000}]},
		{"item_name": "KEMEJA PDH", "size": "L", "prices": [{"year": 2026, "price": 145000}]}
	]}`, http.StatusOK)
	if len(res.ItemsCreated) != 2 || res.ItemsCreated[0].ItemName != "KEMEJA PDH" || *res.ItemsCreated[0].Size != "M" {
		t.Fatalf("created %+v", res.ItemsCreated)
	}
	m, l := res.ItemsCreated[0].Id, res.ItemsCreated[1].Id

	// Again (an import of the same rows): no new items, the price updated.
	again := catalogue(t, `{"client_id": `+cid+`, "upsert_items": [{"item_name": "KEMEJA PDH", "size": "M", "prices": [{"year": 2026, "price": 150000}]}]}`, http.StatusOK)
	if len(again.ItemsCreated) != 0 || len(again.PricesBefore) != 1 || !again.PricesBefore[0].Existed || again.PricesBefore[0].Price != 140000 {
		t.Errorf("re-import: %+v", again)
	}

	// A rise for 2027 across both: nothing existed for that year.
	rise := catalogue(t, `{"client_id": `+cid+`, "set_prices": [
		{"client_item_id": `+itoa(uint(m))+`, "year": 2027, "price": 160000},
		{"client_item_id": `+itoa(uint(l))+`, "year": 2027, "price": 155000}
	]}`, http.StatusOK)
	if len(rise.PricesBefore) != 2 || rise.PricesBefore[0].Existed {
		t.Errorf("rise before: %+v", rise.PricesBefore)
	}
	// Undo it: those prices go.
	catalogue(t, `{"client_id": `+cid+`, "delete_prices": [{"client_item_id": `+itoa(uint(m))+`, "year": 2027}, {"client_item_id": `+itoa(uint(l))+`, "year": 2027}]}`, http.StatusOK)
	if _, ok := priceOf(m, 2027); ok {
		t.Error("the undone 2027 price is still there")
	}

	// Another client's item can't be priced through this client.
	catalogue(t, `{"client_id": `+itoa(uint(other.Id))+`, "set_prices": [{"client_item_id": `+itoa(uint(m))+`, "year": 2026, "price": 1}]}`, http.StatusBadRequest)
	if p, _ := priceOf(m, 2026); p != 150000 {
		t.Errorf("price changed through another client: %d", p)
	}

	// Undo of the first batch: its items go, with their prices.
	gone := catalogue(t, `{"client_id": `+cid+`, "delete_items": [`+itoa(uint(m))+`, `+itoa(uint(l))+`]}`, http.StatusOK)
	if len(gone.ItemsDeleted) != 2 {
		t.Errorf("deleted %+v", gone.ItemsDeleted)
	}
	if _, ok := priceOf(m, 2026); ok {
		t.Error("a deleted item's price is left behind")
	}
	catalogue(t, `{"client_id": `+cid+`}`, http.StatusBadRequest)
	catalogue(t, `{"client_id": 999999, "set_prices": [{"client_item_id": 1, "year": 2026, "price": 1}]}`, http.StatusNotFound)
}
