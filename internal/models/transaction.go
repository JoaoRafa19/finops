package models

import "time"

type CreateTransactionDTO struct {
	UserID      int64
	AccountID   int64
	CategoryID  *int64
	PostedOn    time.Time
	Description string
	Amount      float64
	Direction   string
}
