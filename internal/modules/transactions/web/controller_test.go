package transactions

import (
	"context"
	"finops/internal/models"
	service "finops/internal/services"
	"finops/internal/store"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type transactionServiceStub struct {
	createManualFn     func(ctx context.Context, input models.CreateTransactionDTO) (store.Transaction, error)
	createTransferFn   func(ctx context.Context, input models.CreateTransferDTO) ([]store.Transaction, error)
	listRecentByUserFn func(ctx context.Context, userID int64, limit int32) ([]service.TransactionListItem, error)
}

func (s transactionServiceStub) CreateManual(ctx context.Context, input models.CreateTransactionDTO) (store.Transaction, error) {
	if s.createManualFn != nil {
		return s.createManualFn(ctx, input)
	}
	return store.Transaction{}, nil
}

func (s transactionServiceStub) CreateTransfer(ctx context.Context, input models.CreateTransferDTO) ([]store.Transaction, error) {
	if s.createTransferFn != nil {
		return s.createTransferFn(ctx, input)
	}
	return nil, nil
}

func (s transactionServiceStub) ListRecentByUser(ctx context.Context, userID int64, limit int32) ([]service.TransactionListItem, error) {
	if s.listRecentByUserFn != nil {
		return s.listRecentByUserFn(ctx, userID, limit)
	}
	return nil, nil
}

func (s transactionServiceStub) GetForEdit(_ context.Context, _, _ int64) (store.GetTransactionForEditRow, error) {
	return store.GetTransactionForEditRow{}, nil
}
func (s transactionServiceStub) Update(_ context.Context, _, _ int64, _ service.UpdateTransactionDTO) error {
	return nil
}
func (s transactionServiceStub) Delete(_ context.Context, _, _ int64) error { return nil }
func (s transactionServiceStub) FindDuplicate(_ context.Context, _, _ int64, _ time.Time, _ float64, _, _ string) (service.DuplicateMatch, bool, error) {
	return service.DuplicateMatch{}, false, nil // userID-based; assinatura idêntica
}

type accountServiceStub struct {
	listByUserFn          func(ctx context.Context, userID int64) ([]store.Account, error)
	listSummariesByUserFn func(ctx context.Context, userID int64) ([]service.AccountSummary, error)
	getByIDFn             func(ctx context.Context, userID, accountID int64) (store.Account, error)
	createFn              func(ctx context.Context, createDto service.CreateAccountDTO) (store.Account, error)
	updateFn              func(ctx context.Context, updateDto service.UpdateAccountDTO) (store.Account, error)
}

func (s accountServiceStub) ListByUser(ctx context.Context, userID int64) ([]store.Account, error) {
	if s.listByUserFn != nil {
		return s.listByUserFn(ctx, userID)
	}
	return nil, nil
}

func (s accountServiceStub) ListSummariesByUser(ctx context.Context, userID int64) ([]service.AccountSummary, error) {
	if s.listSummariesByUserFn != nil {
		return s.listSummariesByUserFn(ctx, userID)
	}
	return nil, nil
}

func (s accountServiceStub) GetByID(ctx context.Context, userID, accountID int64) (store.Account, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, userID, accountID)
	}
	return store.Account{}, nil
}

func (s accountServiceStub) Create(ctx context.Context, createDto service.CreateAccountDTO) (store.Account, error) {
	if s.createFn != nil {
		return s.createFn(ctx, createDto)
	}
	return store.Account{}, nil
}

func (s accountServiceStub) Update(ctx context.Context, updateDto service.UpdateAccountDTO) (store.Account, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, updateDto)
	}
	return store.Account{}, nil
}

func (s accountServiceStub) Delete(ctx context.Context, userID, accountID int64) error {
	return nil
}

type categoryServiceStub struct {
	getCategoriesFn  func(ctx context.Context, userID int64) ([]store.Category, error)
	createCategoryFn func(ctx context.Context, dto service.CreateCategoryDTO) (*store.Category, error)
	deleteCategoryFn func(ctx context.Context, userID, categoryID int64) error
}

func (s categoryServiceStub) GetUncategorized(ctx context.Context, userID int64) (int32, error) {
	return 0, nil
}

func (s categoryServiceStub) GetCategoryByID(ctx context.Context, userID, categoryID int64) (store.Category, error) {
	return store.Category{}, nil
}

func (s categoryServiceStub) UpdateCategoryName(ctx context.Context, userID, categoryID int64, name string) error {
	return nil
}

var _ service.CategoryService = categoryServiceStub{}

// DeleteCategory implements [service.CategoryService].
func (s categoryServiceStub) DeleteCategory(ctx context.Context, userID int64, categoryID int64) error {
	if s.deleteCategoryFn != nil {
		return s.deleteCategoryFn(ctx, userID, categoryID)
	}

	return nil
}

func (s categoryServiceStub) GetCategories(ctx context.Context, userID int64) ([]store.Category, error) {
	if s.getCategoriesFn != nil {
		return s.getCategoriesFn(ctx, userID)
	}
	return nil, nil
}

func (s categoryServiceStub) CreateCategory(ctx context.Context, dto service.CreateCategoryDTO) (*store.Category, error) {
	if s.createCategoryFn != nil {
		return s.createCategoryFn(ctx, dto)
	}
	return nil, nil
}

func TestCreateTransactionInvalidPayloadRendersModalWithSubmittedValues(t *testing.T) {
	t.Parallel()

	controller := NewController(
		transactionServiceStub{},
		accountServiceStub{
			listByUserFn: func(ctx context.Context, userID int64) ([]store.Account, error) {
				return []store.Account{
					{ID: 1, Name: "Conta principal", Currency: "BRL", Type: "checking"},
				}, nil
			},
		},
		categoryServiceStub{
			getCategoriesFn: func(ctx context.Context, userID int64) ([]store.Category, error) {
				return []store.Category{
					{ID: 2, Name: "Alimentacao", Kind: string(service.EXPENSE)},
				}, nil
			},
		},
	)

	form := url.Values{
		"description": {"Supermercado"},
		"amount":      {"12,34"},
		"account_id":  {"1"},
		"category_id": {"2"},
		"posted_on":   {""},
		"direction":   {"debit"},
	}

	req := httptest.NewRequest(http.MethodPost, "/transactions",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), models.SessionCtxKey,
		models.Session{
			UserID:    10,
			Email:     "user@example.com",
			CSRFToken: "csrf-token",
		}))

	rec := httptest.NewRecorder()
	controller.CreateTransaction(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.StatusCode)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Supermercado") {
		t.Fatalf("expected submitted description to be preserved")
	}
	if !strings.Contains(body, "12,34") {
		t.Fatalf("expected submitted amount to be preserved")
	}
}

func TestBuildTransactionFormStatePreservesSubmittedFields(t *testing.T) {
	t.Parallel()

	form := url.Values{
		"description": {"Supermercado"},
		"amount":      {"12,34"},
		"account_id":  {"1"},
		"category_id": {"2"},
		"posted_on":   {"2026-04-07"},
		"direction":   {"debit"},
	}

	req := httptest.NewRequest(http.MethodPost, "/transactions", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}

	state := buildTransactionFormState(req, "validation error")

	if state.Description != "Supermercado" {
		t.Fatalf("unexpected description: %q", state.Description)
	}
	if state.Amount != "12,34" {
		t.Fatalf("unexpected amount: %q", state.Amount)
	}
	if state.AccountID != "1" {
		t.Fatalf("unexpected account id: %q", state.AccountID)
	}
	if state.CategoryID != "2" {
		t.Fatalf("unexpected category id: %q", state.CategoryID)
	}
	if state.PostedOn != "2026-04-07" {
		t.Fatalf("unexpected posted_on: %q", state.PostedOn)
	}
	if state.Direction != "debit" {
		t.Fatalf("unexpected direction: %q", state.Direction)
	}
	if state.ErrorMessage != "validation error" {
		t.Fatalf("unexpected error message: %q", state.ErrorMessage)
	}
}

func TestCreateTransferInvalidPayloadRendersModalWithSubmittedValues(t *testing.T) {
	t.Parallel()

	controller := NewController(
		transactionServiceStub{},
		accountServiceStub{
			listByUserFn: func(ctx context.Context, userID int64) ([]store.Account, error) {
				return []store.Account{
					{ID: 1, Name: "Conta origem", Currency: "BRL", Type: "checking"},
					{ID: 2, Name: "Conta destino", Currency: "BRL", Type: "checking"},
				}, nil
			},
		},
		categoryServiceStub{},
	)

	form := url.Values{
		"from_account_id": {"1"},
		"to_account_id":   {"1"},
		"amount":          {"10,50"},
		"posted_on":       {"2026-04-07"},
	}

	req := httptest.NewRequest(http.MethodPost, "/transfers", strings.NewReader(form.Encode()))
	req.MultipartForm = &multipart.Form{Value: form}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), models.SessionCtxKey, models.Session{
		UserID:    10,
		Email:     "user@example.com",
		CSRFToken: "csrf-token",
	}))

	rec := httptest.NewRecorder()
	controller.CreateTransfer(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Result().StatusCode)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "contas de origem e destino devem ser diferentes") {
		t.Fatalf("expected submitted amount to be preserved")
	}
	if !strings.Contains(body, "2026-04-07") {
		t.Fatalf("expected submitted date to be preserved")
	}
}
