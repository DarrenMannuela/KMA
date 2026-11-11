package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func printLayoutRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/print-layout", GetPrintLayouts)
	r.PUT("/print-layout/:type", PutPrintLayout)
	r.DELETE("/print-layout/:type", DeletePrintLayout)
	return r
}

// A layout is saved for one document, listed, replaced by a second save,
// refused when it isn't an object, too big, or for an unknown document,
// and removed again (back to the built-in layout).
func TestPrintLayouts(t *testing.T) {
	r := printLayoutRouter()
	t.Cleanup(func() { Connect().Where("1 = 1").Delete(&dto.PrintLayout{}) })

	w := callOn(t, r, http.MethodPut, "/print-layout/sj", `{"settings": {"logoSize": 80, "font": "georgia"}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	w = callOn(t, r, http.MethodPut, "/print-layout/sj", `{"settings": {"logoSize": 96}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("second save: %d %s", w.Code, w.Body.String())
	}

	w = callOn(t, r, http.MethodGet, "/print-layout", "")
	var list []struct {
		DocType  string         `json:"doc_type"`
		Settings map[string]any `json:"settings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("list: %v %s", err, w.Body.String())
	}
	if len(list) != 1 || list[0].DocType != "sj" || list[0].Settings["logoSize"] != float64(96) || list[0].Settings["font"] != nil {
		t.Fatalf("list after two saves: %s", w.Body.String())
	}

	for _, bad := range []struct{ path, body string }{
		{"/print-layout/sj", `{"settings": [1, 2]}`},
		{"/print-layout/sj", `{"settings": "big"}`},
		{"/print-layout/sj", `{}`},
		{"/print-layout/sj", `{"settings": {"x": "` + strings.Repeat("a", 9000) + `"}}`},
	} {
		if w := callOn(t, r, http.MethodPut, bad.path, bad.body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s should be refused, got %d", bad.body[:min(len(bad.body), 40)], w.Code)
		}
	}
	if w := callOn(t, r, http.MethodPut, "/print-layout/letter", `{"settings": {}}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown document: %d", w.Code)
	}

	if w := callOn(t, r, http.MethodDelete, "/print-layout/sj", ""); w.Code != http.StatusOK {
		t.Fatalf("reset: %d", w.Code)
	}
	w = callOn(t, r, http.MethodGet, "/print-layout", "")
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("after reset: %s", w.Body.String())
	}
}
