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
		v1.POST("/order", handler.PostOrders)
		v1.PATCH("/order/:id", handler.UpdateOrders)
		v1.DELETE("/order/:id", handler.DeleteOrders)

		// Delivery Entry
		v1.GET("/delivery", handler.GetDelivery)
		v1.POST("/delivery", handler.PostDelivery)
		v1.PATCH("/delivery/:id", handler.UpdateDelivery)
		v1.DELETE("/delivery/:id", handler.DeleteDelivery)

		// Supplier Entry
		v1.GET("/supplier", handler.GetSupplier)
		v1.POST("/supplier", handler.PostSupplier)
		v1.GET("/supplier/:id", handler.GetSupplierByID)
		v1.PATCH("/supplier/:id", handler.UpdateSupplier)
		v1.DELETE("/supplier/:id", handler.DeleteSupplier)

		// Production Entry
		v1.GET("/production", handler.GetProduction)
		v1.POST("/production", handler.PostProduction)
		v1.PATCH("/production/:id", handler.UpdateProduction)
		v1.DELETE("/production/:id", handler.DeleteProduction)

		// Operation Entry
		v1.GET("/operation", handler.GetOperation)
		v1.POST("/operation", handler.PostOperation)
		v1.PATCH("/operation/:id", handler.UpdateOperation)
		v1.DELETE("/operation/:id", handler.DeleteOperation)

		// Order-Recap Entry
		v1.GET("/order-recap", handler.GetOrderRecap)
		v1.POST("/order-recap", handler.PostOrderRecap)
		v1.PATCH("/order-recap/:id", handler.UpdateOrderRecap)
		v1.DELETE("/order-recap/:id", handler.DeleteOrderRecap)

		// Surat Jalan Entry
		v1.GET("/surat-jalan", handler.GetSuratJalan)
		v1.POST("/surat-jalan", handler.PostSuratJalan)
		v1.PATCH("/surat-jalan/:id", handler.UpdateSuratJalan)
		v1.DELETE("/surat-jalan/:id", handler.DeleteSuratJalan)

		// Item Entry
		v1.GET("/item", handler.GetItems)
		v1.POST("/item", handler.PostItems)
		v1.PATCH("/item/:id", handler.UpdateItems)
		v1.DELETE("/item/:id", handler.DeleteItems)

		// Delivery Order Entry
		v1.GET("/delivery-order", handler.GetDeliveryOrder)
		v1.POST("/delivery-order", handler.PostDeliveryOrder)
		v1.PATCH("/delivery-order/:id", handler.UpdateDeliveryOrder)
		v1.DELETE("/delivery-order/:id", handler.DeleteDeliveryOrder)
	}

	// Start server on port 8000 to match your OpenAPI 'servers' list
	r.Run(":8000")
}
