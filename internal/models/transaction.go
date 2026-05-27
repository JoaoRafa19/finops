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

type CreateTransferDTO struct {
	UserID        int64
	FromAccountID int64
	ToAccountID   int64
	PostedOn      time.Time
	Amount        float64
}
