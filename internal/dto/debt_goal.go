package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateDebtRequest struct {
	Kind         string     `json:"kind"         binding:"required,oneof=payable receivable"`
	Counterparty string     `json:"counterparty" binding:"required,min=1,max=100"`
	Amount       float64    `json:"amount"       binding:"required,gt=0"`
	WalletID     *uuid.UUID `json:"wallet_id"`
	Note         string     `json:"note"         binding:"max=500"`
	Date         *time.Time `json:"date"`
	DueDate      *time.Time `json:"due_date"`
}

type UpdateDebtRequest struct {
	Counterparty *string    `json:"counterparty" binding:"omitempty,min=1,max=100"`
	Note         *string    `json:"note"         binding:"omitempty,max=500"`
	DueDate      *time.Time `json:"due_date"`
}

type DebtPaymentRequest struct {
	Amount   float64    `json:"amount"    binding:"required,gt=0"`
	WalletID *uuid.UUID `json:"wallet_id"`
	Note     string     `json:"note"      binding:"max=500"`
	Date     *time.Time `json:"date"`
}

type DebtFilterRequest struct {
	Kind   string `form:"kind"   binding:"omitempty,oneof=payable receivable"`
	Status string `form:"status" binding:"omitempty,oneof=open paid"`
}

type DebtMovementResponse struct {
	ID          uuid.UUID       `json:"id"`
	Kind        string          `json:"kind"`
	Amount      float64         `json:"amount"`
	WalletDelta float64         `json:"wallet_delta"`
	Wallet      *TransferWallet `json:"wallet"`
	Date        string          `json:"date"`
	Note        string          `json:"note"`
}

type DebtResponse struct {
	ID           uuid.UUID              `json:"id"`
	Kind         string                 `json:"kind"`
	Counterparty string                 `json:"counterparty"`
	Principal    float64                `json:"principal"`
	PaidAmount   float64                `json:"paid_amount"`
	Remaining    float64                `json:"remaining"`
	Status       string                 `json:"status"`
	Note         string                 `json:"note"`
	DueDate      *string                `json:"due_date"`
	Overdue      bool                   `json:"overdue"`
	Movements    []DebtMovementResponse `json:"movements,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

type DebtSummaryResponse struct {
	TotalPayable    float64        `json:"total_payable"`
	TotalReceivable float64        `json:"total_receivable"`
	Debts           []DebtResponse `json:"debts"`
}

type CreateGoalRequest struct {
	Name         string     `json:"name"          binding:"required,min=1,max=100"`
	TargetAmount float64    `json:"target_amount" binding:"required,gt=0"`
	WalletID     uuid.UUID  `json:"wallet_id"     binding:"required"`
	Deadline     *time.Time `json:"deadline"`
}

type UpdateGoalRequest struct {
	Name         *string    `json:"name"          binding:"omitempty,min=1,max=100"`
	TargetAmount *float64   `json:"target_amount" binding:"omitempty,gt=0"`
	Deadline     *time.Time `json:"deadline"`
}

type GoalResponse struct {
	ID            uuid.UUID      `json:"id"`
	Name          string         `json:"name"`
	Wallet        TransferWallet `json:"wallet"`
	TargetAmount  float64        `json:"target_amount"`
	CurrentAmount float64        `json:"current_amount"`
	Remaining     float64        `json:"remaining"`
	ProgressPct   float64        `json:"progress_pct"`
	Deadline      *string        `json:"deadline"`
	MonthsLeft    *int           `json:"months_left"`
	MonthlyNeeded *float64       `json:"monthly_needed"`
	Achieved      bool           `json:"achieved"`
	AchievedAt    *time.Time     `json:"achieved_at"`
}
