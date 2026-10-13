package dto

// A Bukti Kas: one cash-received voucher (id, date, description) holding
// its own lines — separate from a Kas Bon (FinanceHeader), which records
// cash going OUT for production/operation costs. Bukti Kas records cash
// coming IN instead, so it's deliberately its own header type rather than
// a third kind grafted onto FinanceHeader — mixing a cash-in document into
// a table modeling cash-out receipts would misrepresent what each row
// actually is. Id format: "01/BK/KMA/26" (see BuktiKasId.ts on the
// frontend for the suggestion logic, same pattern as Kas Bon's "01/KB/26").
type BuktiKasHeader struct {
	Id          string `json:"id" gorm:"primaryKey"`
	Date        string `json:"date"`
	Description string `json:"description"`
}
