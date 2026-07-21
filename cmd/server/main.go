package main

import (
	"log"

	"github.com/DarrenMannuela/KMA/internal/database"
	"github.com/DarrenMannuela/KMA/internal/handler"
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

	// err := database.DropAllTables()
	// if err != nil {
	// 	log.Fatalf("Failed to connect to database: %v", err)
	// }

	// // Optional: Log success
	// log.Println("Database connection established and migration complete.")
	r := gin.Default()

	// 1. Serve the raw OpenAPI YAML file (needed for Swagger UI to render)
	// Make sure your file is actually at ./api/openapi.yaml
	r.StaticFile("/docs/kma.yaml", "api/kma.yaml")

	// 2. Swagger UI Route
	// This points the browser UI to the YAML file served above
	url := ginSwagger.URL("http://localhost:8000/docs/kma.yaml")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, url))

	// 3. API V1 Routes Group
	v1 := r.Group("/api/v1")
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

		// Order-Recap Entry
		v1.GET("/invoice", handler.GetInvoice)
		v1.GET("/invoice/*id", handler.GetInvoiceByID)
		v1.POST("/invoice", handler.PostInvoice)
		v1.PATCH("/invoice/*id", handler.UpdateInvoice)
		v1.DELETE("/invoice/*id", handler.DeleteInvoice)

		// Item Entry
		v1.GET("/item", handler.GetItems)
		v1.GET("/item/by-order", handler.GetItemsByOrder)
		v1.POST("/item", handler.PostItems)
		v1.PATCH("/item/:id", handler.UpdateItems)
		v1.DELETE("/item/:id", handler.DeleteItems)

		// Delivey Item Entry
		v1.GET("/delivery-item", handler.GetDeliveryItem)
		v1.POST("/delivery-item", handler.PostDeliveryItem)
		v1.PATCH("/delivery-item/:id", handler.UpdateDeliveryItem)
		v1.DELETE("/delivery-item/:id", handler.DeleteDeliveryItem)
	}

	// Start server on port 8000 to match your OpenAPI 'servers' list
	r.Run(":8000")
}
