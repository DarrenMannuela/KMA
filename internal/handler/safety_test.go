package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DarrenMannuela/KMA/internal/database"
	"github.com/gin-gonic/gin"
)

// The handlers open ./db_data/kma.sqlite and write photos under ./uploads,
// relative to the working directory, so the tests run in a scratch folder
// of their own: nothing here can touch a real database.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "kma-handler-test")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll("db_data", 0o755); err != nil {
		log.Fatal(err)
	}
	if err := database.AutoMigrate(); err != nil {
		log.Fatal(err)
	}
	code := m.Run()
	if sqlDB, err := Connect().DB(); err == nil {
		sqlDB.Close()
	}
	os.RemoveAll(dir)
	os.Exit(code)
}

func router() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/client/:id", GetClientByID)
	r.POST("/client", PostClient)
	r.POST("/supplier", PostSupplier)
	r.GET("/supplier", GetSupplier)
	r.DELETE("/supplier/*id", DeleteSupplier)
	r.POST("/client-item", PostClientItem)
	r.GET("/client-item/:id", GetClientItemByID)
	r.DELETE("/client-item/:id", DeleteClientItem)
	return r
}

func call(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router().ServeHTTP(w, req)
	return w
}

// An id in the URL is a value, never SQL: "999 OR 1=1" used to reach the
// query as a condition and return (or delete) whatever row came first.
func TestIDsInTheURLAreNotSQL(t *testing.T) {
	if w := call(t, http.MethodPost, "/client", `{"client_name":"Test Co"}`); w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("creating a client: %d %s", w.Code, w.Body)
	}
	if w := call(t, http.MethodGet, "/client/"+url.PathEscape("999 OR 1=1"), ""); w.Code != http.StatusNotFound {
		t.Errorf("GET /client/999 OR 1=1: %d %s, want 404", w.Code, w.Body)
	}

	var before []json.RawMessage
	json.Unmarshal(call(t, http.MethodGet, "/supplier", "").Body.Bytes(), &before)
	for _, name := range []string{"Supplier A", "Supplier B"} {
		if w := call(t, http.MethodPost, "/supplier", `{"supplier_name":"`+name+`","supplier_category":"sablon"}`); w.Code >= 300 {
			t.Fatalf("creating a supplier: %d %s", w.Code, w.Body)
		}
	}
	call(t, http.MethodDelete, "/supplier/"+url.PathEscape("999 OR 1=1"), "")
	var suppliers []json.RawMessage
	if err := json.Unmarshal(call(t, http.MethodGet, "/supplier", "").Body.Bytes(), &suppliers); err != nil {
		t.Fatal(err)
	}
	if len(suppliers) != len(before)+2 {
		t.Errorf("DELETE /supplier/999 OR 1=1 left %d suppliers, want %d", len(suppliers), len(before)+2)
	}
}

// A new catalogue item can't name its own photo file: deleting the item
// deletes its photo, so a photo_path from the request could have deleted
// any file the server can reach, the database included.
func TestAnItemCantPointItsPhotoAtOtherFiles(t *testing.T) {
	if w := call(t, http.MethodPost, "/client", `{"client_name":"Photo Co"}`); w.Code >= 300 {
		t.Fatalf("creating a client: %d %s", w.Code, w.Body)
	}
	canary := filepath.Join("db_data", "canary.txt")
	if err := os.WriteFile(canary, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := call(t, http.MethodPost, "/client-item", `{"client_id":1,"item_name":"shirt","photo_path":"/db_data/canary.txt"}`)
	if w.Code >= 300 {
		t.Fatalf("creating an item: %d %s", w.Code, w.Body)
	}
	var item struct {
		ID        uint64  `json:"id"`
		PhotoPath *string `json:"photo_path"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.PhotoPath != nil {
		t.Errorf("photo_path from the request was kept: %q", *item.PhotoPath)
	}
	call(t, http.MethodDelete, "/client-item/1", "")
	if _, err := os.Stat(canary); err != nil {
		t.Errorf("deleting the item deleted %s", canary)
	}
}

func TestPhotoFilePathStaysInThePhotoFolder(t *testing.T) {
	for path, want := range map[string]string{
		"/uploads/client-items/5.png?v=1721800000000":    filepath.Join(clientItemPhotoDir, "5.png"),
		"/uploads/client-items/12.jpg":                   filepath.Join(clientItemPhotoDir, "12.jpg"),
		"/db_data/kma.sqlite":                            "",
		"/uploads/client-items/../../db_data/kma.sqlite": "",
		"/uploads/client-items/..":                       "",
		"/uploads/client-items/":                         "",
		"uploads/client-items/5.png":                     "",
		"/uploads/other/5.png":                           "",
		`/uploads/client-items/..\kma.sqlite`:            "",
	} {
		got, ok := photoFilePath(path)
		if want == "" && ok {
			t.Errorf("photoFilePath(%q) = %q, want refused", path, got)
		}
		if want != "" && (!ok || got != want) {
			t.Errorf("photoFilePath(%q) = %q, %v, want %q", path, got, ok, want)
		}
	}
}
