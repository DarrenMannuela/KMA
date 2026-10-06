package handler

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetBudgets(c *gin.Context) {
	var budgets []dto.Budget
	if err := Connect().Order("scope, category").Find(&budgets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, budgets)
}

// PutBudget sets the monthly budget for a scope and category; an amount of 0
// removes it.
func PutBudget(c *gin.Context) {
	var b dto.Budget
	if err := c.ShouldBindBodyWithJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	b.Category = strings.TrimSpace(b.Category)
	if b.Scope != "production" && b.Scope != "operation" {
		c.JSON(http.StatusBadRequest, gin.H{"error": `scope must be "production" or "operation"`})
		return
	}
	if b.Amount < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A budget can't be negative"})
		return
	}
	db := Connect()
	if b.Amount == 0 {
		if err := db.Where("scope = ? AND category = ?", b.Scope, b.Category).Delete(&dto.Budget{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
			return
		}
		c.Status(http.StatusNoContent)
		return
	}
	b.Id = 0
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "scope"}, {Name: "category"}},
		DoUpdates: clause.AssignmentColumns([]string{"amount"}),
	}).Create(&b).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Saving failed"})
		return
	}
	db.Where("scope = ? AND category = ?", b.Scope, b.Category).First(&b)
	c.JSON(http.StatusOK, b)
}

func GetRecurringCosts(c *gin.Context) {
	var costs []dto.RecurringCost
	if err := Connect().Order("category, description").Find(&costs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, costs)
}

func PostRecurringCost(c *gin.Context) {
	var rc dto.RecurringCost
	if err := c.ShouldBindBodyWithJSON(&rc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	rc.Id = 0
	if msg := checkRecurring(&rc); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if err := Connect().Create(&rc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
		return
	}
	c.JSON(http.StatusCreated, rc)
}

func UpdateRecurringCost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	db := Connect()
	var rc dto.RecurringCost
	if err := db.Where("id = ?", id).First(&rc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Recurring cost not found"})
		return
	}
	if err := c.ShouldBindBodyWithJSON(&rc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	rc.Id = uint(id)
	if msg := checkRecurring(&rc); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if err := db.Save(&rc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}
	c.JSON(http.StatusOK, rc)
}

func DeleteRecurringCost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	res := Connect().Where("id = ?", id).Delete(&dto.RecurringCost{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Recurring cost not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func checkRecurring(rc *dto.RecurringCost) string {
	rc.Category = strings.TrimSpace(rc.Category)
	rc.Description = strings.TrimSpace(rc.Description)
	switch {
	case rc.Category == "":
		return "A recurring cost needs a category"
	case rc.Price <= 0:
		return "A recurring cost needs a price"
	case rc.LastPosted != "" && !monthPattern.MatchString(rc.LastPosted):
		return "last_posted must look like 2026-10"
	}
	return ""
}

var monthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

type postRecurringRequest struct {
	Month    string `json:"month"`     // "2026-10"
	HeaderId string `json:"header_id"` // the new Kas Bon's ID
	Date     string `json:"date"`      // its date, in that month
}

// PostRecurringMonth adds one Kas Bon for the month holding a line for every
// active recurring cost not yet posted for it. Posting the same month again
// adds nothing.
func PostRecurringMonth(c *gin.Context) {
	var req postRecurringRequest
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	req.HeaderId = strings.TrimSpace(req.HeaderId)
	if !monthPattern.MatchString(req.Month) || req.HeaderId == "" || !strings.HasPrefix(req.Date, req.Month) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "month (2026-10), header_id and a date in that month are required"})
		return
	}
	var posted []dto.OperationItem
	err := Connect().Transaction(func(tx *gorm.DB) error {
		var due []dto.RecurringCost
		if err := tx.Where("active = ? AND (last_posted = '' OR last_posted IS NULL OR last_posted < ?)", true, req.Month).
			Order("category, description").Find(&due).Error; err != nil {
			return err
		}
		if len(due) == 0 {
			return nil
		}
		var n int64
		tx.Model(&dto.FinanceHeader{}).Where("id = ?", req.HeaderId).Count(&n)
		if n > 0 {
			return batchError{http.StatusConflict, fmt.Sprintf("Kas Bon %s already exists", req.HeaderId)}
		}
		h := dto.FinanceHeader{Id: req.HeaderId, Date: req.Date, Description: "RECURRING COSTS " + req.Month}
		if err := tx.Create(&h).Error; err != nil {
			return err
		}
		for _, rc := range due {
			desc := rc.Description
			if desc == "" {
				desc = rc.Category
			}
			item := dto.OperationItem{HeaderId: h.Id, Category: rc.Category, Description: desc, Price: rc.Price}
			if err := tx.Omit("Header").Create(&item).Error; err != nil {
				return err
			}
			posted = append(posted, item)
			if err := tx.Model(&dto.RecurringCost{}).Where("id = ?", rc.Id).Update("last_posted", req.Month).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if be, ok := err.(batchError); ok {
		c.JSON(be.status, gin.H{"error": be.msg})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Posting failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"posted": posted})
}
