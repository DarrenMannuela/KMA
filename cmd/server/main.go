package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DarrenMannuela/KMA/internal/database"
	"github.com/DarrenMannuela/KMA/internal/handler"
	mw "github.com/DarrenMannuela/KMA/internal/middleware"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	err := database.AutoMigrate()
	if err != nil {
		// If DB fails, we stop the server immediately
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Open the shared connection now, so a database that can't be opened stops
	// the server at start rather than mid-request.
	handler.Connect()

	r := gin.Default()

	// The OpenAPI spec, for the Swagger UI below.
	r.StaticFile("/docs/kma.yaml", "api/kma.yaml")

	// Catalogue photos (written by UploadClientItemPhoto), behind RequireAuth
	// like the API: they're client-confidential.
	uploads := r.Group("/uploads")
	uploads.Use(mw.RequireAuth())
	uploads.Static("/", "./uploads")

	// 2. Swagger UI Route
	// This points the browser UI to the YAML file served above
	url := ginSwagger.URL("http://localhost:8000/docs/kma.yaml")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, url))

	// Outside the auth group: "is the backend up", whether or not the caller
	// is logged in.
	r.GET("/api/v1/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// RequireAuth is the security boundary: every request's session is checked
	// with the auth service before it reaches a handler.
	v1 := r.Group("/api/v1")
	v1.Use(mw.RequireAuth())
	{
		// Order Entry
		v1.GET("/order", handler.GetOrders)
		v1.GET("/order/*id", handler.GetOrderByID)
		v1.POST("/order", handler.PostOrders)
		v1.PATCH("/order/*id", handler.UpdateOrders)
		v1.DELETE("/order/*id", handler.DeleteOrders)

		// Delivery Entry
		v1.GET("/delivery", handler.GetDelivery)
		v1.GET("/delivery/*id", handler.GetDeliveryByID)
		v1.POST("/delivery", handler.PostDelivery)
		v1.PATCH("/delivery/*id", handler.UpdateDelivery)
		v1.DELETE("/delivery/*id", handler.DeleteDelivery)

		// Supplier Entry
		v1.GET("/supplier", handler.GetSupplier)
		v1.POST("/supplier", handler.PostSupplier)
		v1.GET("/supplier/*id", handler.GetSupplierByID)
		v1.PATCH("/supplier/*id", handler.UpdateSupplier)
		v1.DELETE("/supplier/*id", handler.DeleteSupplier)

		// Finance Header Entry — shared parent for Production and Operation
		// Kas Bons. Filter by type with ?type=production or ?type=operation.
		v1.GET("/finance-header", handler.GetFinanceHeaders)
		v1.GET("/finance-header/*id", handler.GetFinanceHeaderByID)
		v1.POST("/finance-header", handler.PostFinanceHeader)
		v1.PATCH("/finance-header/*id", handler.UpdateFinanceHeader)
		v1.DELETE("/finance-header/*id", handler.DeleteFinanceHeader)

		// Production Item Entry — material lines under a production header
		v1.GET("/production-item", handler.GetProductionItems)
		v1.GET("/production-item/by-header", handler.GetProductionItemsByHeader)
		v1.GET("/production-item/grouped", handler.GetProductionItemsGrouped)
		v1.POST("/production-item", handler.PostProductionItem)
		v1.PATCH("/production-item/:id", handler.UpdateProductionItem)
		v1.DELETE("/production-item/:id", handler.DeleteProductionItem)

		// Operation Item Entry — cost lines under an operation header
		v1.GET("/operation-item", handler.GetOperationItems)
		v1.GET("/operation-item/by-header", handler.GetOperationItemsByHeader)
		v1.GET("/operation-item/grouped", handler.GetOperationItemsGrouped)
		v1.POST("/operation-item", handler.PostOperationItem)
		v1.PATCH("/operation-item/:id", handler.UpdateOperationItem)
		v1.DELETE("/operation-item/:id", handler.DeleteOperationItem)

		// Kas Bon changes in one transaction (new Kas Bon, paste, import,
		// bulk edit, undo), monthly budgets and recurring costs.
		v1.POST("/finance/batch", handler.PostFinanceBatch)
		v1.GET("/budget", handler.GetBudgets)
		v1.PUT("/budget", handler.PutBudget)
		v1.GET("/recurring-cost", handler.GetRecurringCosts)
		v1.POST("/recurring-cost", handler.PostRecurringCost)
		v1.POST("/recurring-cost/post-month", handler.PostRecurringMonth)
		v1.PATCH("/recurring-cost/:id", handler.UpdateRecurringCost)
		v1.DELETE("/recurring-cost/:id", handler.DeleteRecurringCost)

		// Order-Recap Entry
		v1.GET("/invoice", handler.GetInvoice)
		v1.GET("/invoice/*id", handler.GetInvoiceByID)
		v1.POST("/invoice", handler.PostInvoice)
		v1.PATCH("/invoice/*id", handler.UpdateInvoice)
		v1.DELETE("/invoice/*id", handler.DeleteInvoice)

		// Item Entry
		v1.GET("/item", handler.GetItems)
		v1.GET("/item/by-order", handler.GetItemsByOrder)
		v1.GET("/item/:id", handler.GetItemByID)
		v1.POST("/item", handler.PostItems)
		v1.PATCH("/item/:id", handler.UpdateItems)
		v1.DELETE("/item/:id", handler.DeleteItems)

		// Delivey Item Entry
		v1.GET("/delivery-item", handler.GetDeliveryItem)
		v1.POST("/delivery-item", handler.PostDeliveryItem)
		v1.PATCH("/delivery-item/:id", handler.UpdateDeliveryItem)
		v1.DELETE("/delivery-item/:id", handler.DeleteDeliveryItem)

		// Client Entry — the company; Orders/Deliveries optionally link
		// to one via client_id (see Orders.go), but stay free-text-able.
		v1.GET("/client", handler.GetClients)
		v1.GET("/client/:id", handler.GetClientByID)
		v1.POST("/client", handler.PostClient)
		v1.PATCH("/client/:id", handler.UpdateClient)
		v1.DELETE("/client/:id", handler.DeleteClient)

		// Client Contact Entry — one-to-many POCs under a client, for
		// clients with multiple locations/departments.
		v1.GET("/client-contact", handler.GetClientContacts)
		v1.GET("/client-contact/by-client", handler.GetClientContactsByClient)
		v1.GET("/client-contact/:id", handler.GetClientContactByID)
		v1.POST("/client-contact", handler.PostClientContact)
		v1.PATCH("/client-contact/:id", handler.UpdateClientContact)
		v1.DELETE("/client-contact/:id", handler.DeleteClientContact)

		// Client Item Entry — each client's catalogue is fully
		// independent (no shared master price list).
		v1.GET("/client-item", handler.GetClientItems)
		v1.GET("/client-item/by-client", handler.GetClientItemsByClient)
		v1.GET("/client-item/:id", handler.GetClientItemByID)
		v1.POST("/client-item", handler.PostClientItem)
		v1.PATCH("/client-item/:id", handler.UpdateClientItem)
		v1.DELETE("/client-item/:id", handler.DeleteClientItem)
		v1.POST("/client-item/:id/photo", handler.UploadClientItemPhoto)
		v1.DELETE("/client-item/:id/photo", handler.DeleteClientItemPhoto)

		// One row per (client item, year); /grouped returns { [client_item_id]: Price[] }.
		v1.GET("/client-item-price", handler.GetClientItemPrices)
		v1.GET("/client-item-price/by-item", handler.GetClientItemPricesByItem)
		v1.GET("/client-item-price/grouped", handler.GetClientItemPricesGrouped)
		v1.POST("/client-item-price", handler.PostClientItemPrice)
		v1.PATCH("/client-item-price/:id", handler.UpdateClientItemPrice)
		v1.DELETE("/client-item-price/:id", handler.DeleteClientItemPrice)
	}

	// An http.Server so a stop is graceful: on SIGTERM (every Docker stop and
	// update) it stops taking requests, lets those in flight finish (up to 20s;
	// compose allows 30s), then closes the database, folding the WAL back in.
	// ReadHeaderTimeout drops clients that never finish their headers; there's
	// no overall read timeout, since a photo upload on a slow phone takes a while.
	srv := &http.Server{
		Addr:              ":8000",
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	}()
	log.Println("Listening on :8000")

	<-stop.Done()
	log.Println("Stopping: finishing the requests in progress")
	ctx, done := context.WithTimeout(context.Background(), 20*time.Second)
	defer done()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Stopped before every request finished: %v", err)
	}
	if sqlDB, err := handler.Connect().DB(); err == nil {
		if err := sqlDB.Close(); err != nil {
			log.Printf("Closing the database: %v", err)
		}
	}
	log.Println("Stopped cleanly")
}
