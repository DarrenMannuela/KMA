package dto

// Budget is a monthly spending limit. Scope is "production" or "operation";
// Category is an operation category or a supplier category, or "" for the
// whole scope.
type Budget struct {
	Id       uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Scope    string `json:"scope" gorm:"uniqueIndex:idx_budget_scope_category;not null"`
	Category string `json:"category" gorm:"uniqueIndex:idx_budget_scope_category"`
	Amount   int64  `json:"amount"`
}

// RecurringCost is an operation cost that comes back every month (rent,
// salaries, internet). Posting a month adds one Kas Bon with a line for each
// active cost; LastPosted ("2026-10") keeps a month from being posted twice.
type RecurringCost struct {
	Id          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Price       int64  `json:"price"`
	Active      bool   `json:"active"`
	LastPosted  string `json:"last_posted"`
}
