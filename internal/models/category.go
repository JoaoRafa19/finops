package models

type CategoryKind string

const (
	EXPENSE  CategoryKind = "expense"
	INCOME   CategoryKind = "income"
	TRANSFER CategoryKind = "transfer"
)

type CreateCategoryDTO struct {
	Kind   CategoryKind `json:"kind"`
	UserID int64        `json:"user_id"`
	Name   string       `json:"name"`
}


type CategoryResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

