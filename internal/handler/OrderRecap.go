package handler

import (
	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetOrderRecap(c *gin.Context) {
	var orderRecap []dto.OrderRecap
	db := Connect()

	results := db.Find(&orderRecap)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
	}

	c.JSON(200, orderRecap)

}

// func PostOrderRecap(c *gin.Context) {
// 	var newOrder dto.OrderRecap

// }
