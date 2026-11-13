package handler

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func trashRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/order/*id", DeleteOrders)
	r.DELETE("/invoice/*id", DeleteInvoice)
	r.DELETE("/delivery/*id", DeleteDelivery)
	r.DELETE("/client/:id", DeleteClient)
	r.DELETE("/supplier/*id", DeleteSupplier)
	r.POST("/trash/:id/restore", RestoreDeleted)
	return r
}

func seed(t *testing.T, rows ...any) {
	t.Helper()
	for _, row := range rows {
		if err := Connect().Omit(clause.Associations).Create(row).Error; err != nil {
			t.Fatalf("seed %T: %v", row, err)
		}
	}
}

func count(t *testing.T, model any, where string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := Connect().Model(model).Where(where, args...).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// deleteForRestore deletes and returns the restore id the 204 carries.
func deleteForRestore(t *testing.T, r *gin.Engine, path string) string {
	t.Helper()
	w := callOn(t, r, http.MethodDelete, path, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete %s: %d %s", path, w.Code, w.Body.String())
	}
	id := w.Header().Get(restoreHeader)
	if id == "" {
		t.Fatalf("delete %s: no %s header", path, restoreHeader)
	}
	return id
}

func restore(t *testing.T, r *gin.Engine, id string, want int) {
	t.Helper()
	if w := callOn(t, r, http.MethodPost, "/trash/"+id+"/restore", ""); w.Code != want {
		t.Fatalf("restore %s: %d, want %d: %s", id, w.Code, want, w.Body.String())
	}
}

// Deleting a client and then an order of theirs, and undoing both, puts back
// every row the deletes removed, relinks what they unlinked, and can't be
// done twice.
func TestDeleteAndRestore(t *testing.T) {
	r := trashRouter()
	date := time.Date(2091, 1, 5, 0, 0, 0, 0, time.UTC)
	size := "L"

	client := dto.Client{ClientName: "TRASH TEST CLIENT"}
	seed(t, &client)
	contact := dto.ClientContact{ClientId: client.Id, Name: "BUDI", IsPrimary: true}
	catItem := dto.ClientItem{ClientId: client.Id, ItemName: "KEMEJA", Size: &size}
	seed(t, &contact, &catItem)
	seed(t, &dto.ClientItemPrice{ClientItemId: catItem.Id, Year: 2091, Price: 125000})

	orderId := "950/KMA/91"
	seed(t, &dto.Orders{Id: orderId, Date: date, ClientId: &client.Id, ClientContactId: &contact.Id})
	seed(t, &dto.Items{OrderId: orderId, ItemName: "KEMEJA", Size: &size, Amount: 10, Price: 125000, SubTotal: 1250000})
	seed(t, &dto.Invoice{Id: "950/INV/91", OrderId: orderId, Type: "dp", Total: 1250000, Tanggal: date, Status: "unpaid"})
	seed(t, &dto.InvoiceSheet{InvoiceId: "950/INV/91", Settings: `{"rows":{"total":{"bold":true}}}`, UpdatedAt: date})
	seed(t, &dto.Delivery{Id: "950/DO/91", Type: "do", Address: "JL TEST", Date: date, OrderId: &orderId})
	seed(t, &dto.DeliveryItem{DeliveryId: "950/DO/91", ItemName: "KEMEJA", Amount: 10})
	res := batch(t, `{
		"new_headers": [{"id": "05/KB/91", "date": "2091-01-06"}],
		"production_create": [{"header_id": "05/KB/91", "material_name": "TRASH TEST LINE", "price": 1000, "amount": 3, "order_id": "950/KMA/91", "supplier_id": `+testSupplier(t)+`}]
	}`, http.StatusOK)
	lineId := res.Production[0].Id

	// The client: contacts and catalogue go with it, the order is unlinked.
	clientRestore := deleteForRestore(t, r, "/client/"+itoa(uint(client.Id)))
	if count(t, &dto.ClientItemPrice{}, "client_item_id = ?", catItem.Id) != 0 || count(t, &dto.ClientContact{}, "client_id = ?", client.Id) != 0 {
		t.Fatal("client delete didn't cascade")
	}
	if count(t, &dto.Orders{}, "id = ? AND client_id IS NULL", orderId) != 1 {
		t.Fatal("client delete didn't unlink the order")
	}

	// The order: items, invoice and delivery go with it, the Kas Bon line is unlinked.
	orderRestore := deleteForRestore(t, r, "/order/"+url.PathEscape(orderId))
	for model, where := range map[any]string{&dto.Items{}: "order_id = ?", &dto.Invoice{}: "order_id = ?", &dto.Delivery{}: "order_id = ?"} {
		if count(t, model, where, orderId) != 0 {
			t.Fatalf("order delete left %T rows", model)
		}
	}
	if count(t, &dto.DeliveryItem{}, "delivery_id = ?", "950/DO/91") != 0 {
		t.Fatal("order delete left delivery items")
	}
	if count(t, &dto.InvoiceSheet{}, "invoice_id = ?", "950/INV/91") != 0 {
		t.Fatal("order delete left its invoice's formatting")
	}

	// Undo the client first, then the order: both relink in either order.
	restore(t, r, clientRestore, http.StatusOK)
	if count(t, &dto.ClientContact{}, "id = ? AND is_primary = ?", contact.Id, true) != 1 || count(t, &dto.ClientItemPrice{}, "client_item_id = ? AND price = ?", catItem.Id, 125000) != 1 {
		t.Error("client restore lost contacts or prices")
	}
	restore(t, r, orderRestore, http.StatusOK)
	if count(t, &dto.Orders{}, "id = ?", orderId) != 1 || count(t, &dto.Items{}, "order_id = ? AND amount = ?", orderId, 10) != 1 ||
		count(t, &dto.Invoice{}, "id = ? AND total = ?", "950/INV/91", 1250000) != 1 ||
		count(t, &dto.DeliveryItem{}, "delivery_id = ?", "950/DO/91") != 1 {
		t.Error("order restore lost rows")
	}
	if count(t, &dto.InvoiceSheet{}, "invoice_id = ? AND settings LIKE ?", "950/INV/91", "%bold%") != 1 {
		t.Error("order restore lost its invoice's formatting")
	}
	if count(t, &dto.ProductionItem{}, "id = ? AND order_id = ?", lineId, orderId) != 1 {
		t.Error("order restore didn't relink its Kas Bon line")
	}
	// The order was deleted while the client was gone, so it came back unlinked;
	// the client's restore had nothing of its to relink at that point.
	restore(t, r, orderRestore, http.StatusNotFound) // a second Undo finds nothing

	// An invoice can't come back without its order.
	invRestore := deleteForRestore(t, r, "/invoice/"+url.PathEscape("950/INV/91"))
	if count(t, &dto.InvoiceSheet{}, "invoice_id = ?", "950/INV/91") != 0 {
		t.Fatal("invoice delete left its formatting")
	}
	deleteForRestore(t, r, "/order/"+url.PathEscape(orderId))
	restore(t, r, invRestore, http.StatusConflict)

	// An order whose client was deleted after it comes back, just unlinked.
	other := dto.Client{ClientName: "TRASH TEST CLIENT 2"}
	seed(t, &other)
	seed(t, &dto.Orders{Id: "951/KMA/91", Date: date, ClientId: &other.Id})
	o2 := deleteForRestore(t, r, "/order/"+url.PathEscape("951/KMA/91"))
	deleteForRestore(t, r, "/client/"+itoa(uint(other.Id)))
	restore(t, r, o2, http.StatusOK)
	if count(t, &dto.Orders{}, "id = ? AND client_id IS NULL", "951/KMA/91") != 1 {
		t.Error("an order whose client is gone should come back unlinked")
	}

	// A deleted supplier comes back with its catalogue and documents.
	sup := dto.Supplier{SupplierName: "TRASH TEST SUPPLIER", SupplierCategory: "general_supplier"}
	seed(t, &sup)
	brand := "ACME"
	mat := dto.SupplierMaterial{SupplierId: sup.Id, Name: "TEST TWILL", Brand: &brand, Price: 40000}
	seed(t, &mat)
	seed(t, &dto.SupplierMaterialColor{SupplierMaterialId: mat.Id, Code: "01", Hex: "#112233"},
		&dto.SupplierDocument{SupplierId: sup.Id, FileName: "Price list.pdf", FilePath: "/uploads/supplier-documents/x.pdf", UploadedAt: date})
	supRestore := deleteForRestore(t, r, "/supplier/"+itoa(uint(sup.Id)))
	if count(t, &dto.SupplierMaterial{}, "supplier_id = ?", sup.Id) != 0 {
		t.Fatal("supplier delete didn't cascade")
	}
	restore(t, r, supRestore, http.StatusOK)
	if count(t, &dto.SupplierMaterialColor{}, "supplier_material_id = ?", mat.Id) != 1 || count(t, &dto.SupplierDocument{}, "supplier_id = ?", sup.Id) != 1 {
		t.Error("supplier restore lost its catalogue or documents")
	}

	// Deleting what isn't there is still a 404, with nothing kept.
	if w := callOn(t, r, http.MethodDelete, "/delivery/NOPE", ""); w.Code != http.StatusNotFound || w.Header().Get(restoreHeader) != "" {
		t.Errorf("missing delivery: %d, header %q", w.Code, w.Header().Get(restoreHeader))
	}
}
