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
	// err := database.AutoMigrate()
	// if err != nil {
	// 	// If DB fails, we stop the server immediately
	// 	log.Fatalf("Failed to connect to database: %v", err)
	// }

	err := database.DropAllTables()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

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
		v1.GET("/order", func(c *gin.Context) { c.JSON(200, gin.H{"message": "get all orders"}) })
		v1.POST("/order", func(c *gin.Context) { c.JSON(201, gin.H{"status": "order created"}) })

		// Delivery Entry
		v1.GET("/delivery", func(c *gin.Context) { c.JSON(200, gin.H{"message": "get all deliveries"}) })

		// Supplier Entry
		v1.GET("/supplier", handler.GetSupplier)
		v1.POST("/supplier", handler.SetSupplier)
		v1.GET("/supplier/:id", handler.GetSupplierByID)
		v1.PATCH("/supplier/:id", handler.UpdateSupplier)
		v1.DELETE("/supplier/:id", handler.DeleteSupplier)

		// Production Entry
		v1.GET("/production-entry", func(c *gin.Context) { c.JSON(200, gin.H{"message": "get production"}) })

		// Operation Entry
		v1.GET("/operation-entry", func(c *gin.Context) { c.JSON(200, gin.H{"message": "get operations"}) })
	}

	// Start server on port 8000 to match your OpenAPI 'servers' list
	r.Run(":8000")
}
