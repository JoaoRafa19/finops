package category

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"finops/internal/models"
	service "finops/internal/services"
	"finops/internal/store"
)

type categoryServiceStub struct {
	createFn func(ctx context.Context, dto service.CreateCategoryDTO) (*store.Category, error)
}

func (s categoryServiceStub) GetCategories(_ context.Context, _ int64) ([]store.Category, error) {
	return []store.Category{{ID: 1, Name: "Alimentação", Kind: "expense"}}, nil
}
func (s categoryServiceStub) GetCategoryByID(_ context.Context, _, _ int64) (store.Category, error) {
	return store.Category{}, nil
}
func (s categoryServiceStub) CreateCategory(ctx context.Context, dto service.CreateCategoryDTO) (*store.Category, error) {
	if s.createFn != nil {
		return s.createFn(ctx, dto)
	}
	return &store.Category{ID: 2, Name: dto.Name, Kind: string(dto.Kind)}, nil
}
func (s categoryServiceStub) UpdateCategoryName(_ context.Context, _, _ int64, _ string) error {
	return nil
}
func (s categoryServiceStub) DeleteCategory(_ context.Context, _, _ int64) error { return nil }
func (s categoryServiceStub) GetUncategorized(_ context.Context, _ int64) (int32, error) {
	return 0, nil
}

func postCreateCategory(t *testing.T, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	c := NewCategoryController(categoryServiceStub{})

	req := httptest.NewRequest("POST", "/categories", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), models.SessionCtxKey, models.Session{
		UserID:    1,
		Email:     "user@example.com",
		CSRFToken: "csrf-token",
	}))

	rec := httptest.NewRecorder()
	c.CreateCategory(rec, req)
	return rec
}

// Sucesso deve disparar o evento que fecha o modal e atualizar o painel via OOB.
func TestCreateCategorySuccessTriggersModalClose(t *testing.T) {
	rec := postCreateCategory(t, url.Values{
		"name":          {"Mercado"},
		"category_kind": {"expense"},
	})

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("HX-Trigger-After-Swap"); got != "category-created" {
		t.Errorf("HX-Trigger-After-Swap = %q, want category-created", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="categories-panel"`) || !strings.Contains(body, `hx-swap-oob`) {
		t.Error("success response should contain OOB categories panel")
	}
}

// Erro de validação deve re-renderizar o modal (sem evento de fechamento).
func TestCreateCategoryInvalidKindKeepsModalOpen(t *testing.T) {
	rec := postCreateCategory(t, url.Values{
		"name":          {"Mercado"},
		"category_kind": {"invalido"},
	})

	if got := rec.Header().Get("HX-Trigger-After-Swap"); got != "" {
		t.Errorf("error response must not trigger category-created, got %q", got)
	}
	if !strings.Contains(rec.Body.String(), "invalid kind") {
		t.Error("error response should show validation message")
	}
}
