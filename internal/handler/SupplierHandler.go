package handler

import (
	"github.com/DarrenMannuela/KMA/internal/handler"
	"github.com/gin-gonic/gin"
)

func GetSupplier() error {
	db, err := handler.Connect()
	if err != nil {
		return err
	}

}

func SetSupplier(c *gin.Context) error {

}
