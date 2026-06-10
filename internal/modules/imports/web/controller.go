package web

import (
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
	"strconv"
	"strings"
)

type ImportController struct {
	importSvc  service.ImportService
	accountSvc service.AccountService
}

func NewImportController(importSvc service.ImportService, accountSvc service.AccountService) *ImportController {
	return &ImportController{importSvc: importSvc, accountSvc: accountSvc}
}

func (c *ImportController) Page(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	accounts, err := c.accountSvc.ListByUser(r.Context(), session.UserID)
	if err != nil {
		logger.Error("import_page_accounts_failed", "user_id", session.UserID, "error", err)
		http.Error(w, "erro ao carregar contas", http.StatusInternalServerError)
		return
	}

	render.Templ(w, r, http.StatusOK, templates.ImportPage(accounts, session.CSRFToken))
}

func (c *ImportController) Upload(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		render.Templ(w, r, http.StatusBadRequest, templates.ImportErrorFragment("Arquivo muito grande ou inválido."))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		render.Templ(w, r, http.StatusBadRequest, templates.ImportErrorFragment("Nenhum arquivo enviado."))
		return
	}
	defer file.Close()

	accountID := r.FormValue("account_id")
	if accountID == "" {
		render.Templ(w, r, http.StatusBadRequest, templates.ImportErrorFragment("Selecione uma conta."))
		return
	}

	name := strings.ToLower(header.Filename)
	isOFX := strings.HasSuffix(name, ".ofx") || strings.HasSuffix(name, ".qfx")

	if isOFX {
		rows, err := c.importSvc.ParseOFX(file)
		if err != nil {
			logger.Warn("import_ofx_parse_failed", "user_id", session.UserID, "error", err)
			render.Templ(w, r, http.StatusOK, templates.ImportErrorFragment("Erro ao processar arquivo OFX: "+err.Error()))
			return
		}
		uuid := c.importSvc.StorePreview(rows)
		render.Templ(w, r, http.StatusOK, templates.OFXPreviewFragment(rows, uuid, session.CSRFToken, accountID))
		return
	}

	// CSV: retorna formulário de mapeamento
	headers, sample, err := c.importSvc.ParseCSVHeaders(file)
	if err != nil {
		logger.Warn("import_csv_headers_failed", "user_id", session.UserID, "error", err)
		render.Templ(w, r, http.StatusOK, templates.ImportErrorFragment("Erro ao ler CSV: "+err.Error()))
		return
	}

	// Armazena dados brutos temporariamente para o preview-csv poder reler
	// (re-parse acontece no preview-csv com o mapeamento)
	render.Templ(w, r, http.StatusOK, templates.CSVMappingFragment(headers, sample, session.CSRFToken, accountID))
}

func (c *ImportController) PreviewCSV(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		render.Templ(w, r, http.StatusBadRequest, templates.ImportErrorFragment("Arquivo muito grande ou inválido."))
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		render.Templ(w, r, http.StatusBadRequest, templates.ImportErrorFragment("Arquivo não encontrado."))
		return
	}
	defer file.Close()

	mapping := service.CSVColumnMap{
		DateCol:      parseCol(r.FormValue("date_col")),
		DateFormat:   r.FormValue("date_format"),
		DescCol:      parseCol(r.FormValue("desc_col")),
		AmountCol:    parseCol(r.FormValue("amount_col")),
		SignedAmount:  r.FormValue("signed_amount") == "true",
		DebitCol:     parseCol(r.FormValue("debit_col")),
		CreditCol:    parseCol(r.FormValue("credit_col")),
	}
	if mapping.DateFormat == "" {
		mapping.DateFormat = "02/01/2006"
	}

	rows, err := c.importSvc.ParseCSVRows(file, mapping)
	if err != nil {
		logger.Warn("import_csv_rows_failed", "user_id", session.UserID, "error", err)
		render.Templ(w, r, http.StatusOK, templates.ImportErrorFragment("Erro ao processar CSV: "+err.Error()))
		return
	}
	if len(rows) == 0 {
		render.Templ(w, r, http.StatusOK, templates.ImportErrorFragment("Nenhuma linha válida encontrada com o mapeamento escolhido."))
		return
	}

	accountID := r.FormValue("account_id")
	uuid := c.importSvc.StorePreview(rows)
	render.Templ(w, r, http.StatusOK, templates.CSVPreviewFragment(rows, uuid, session.CSRFToken, accountID))
}

func (c *ImportController) Confirm(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		render.Templ(w, r, http.StatusBadRequest, templates.ImportErrorFragment("Requisição inválida."))
		return
	}

	uuid := r.FormValue("uuid")
	accountIDStr := r.FormValue("account_id")

	rows, ok := c.importSvc.LoadPreview(uuid)
	if !ok {
		render.Templ(w, r, http.StatusOK, templates.ImportErrorFragment("Sessão de importação expirada. Faça o upload novamente."))
		return
	}

	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
	if err != nil {
		render.Templ(w, r, http.StatusBadRequest, templates.ImportErrorFragment("Conta inválida."))
		return
	}

	result, err := c.importSvc.ImportRows(r.Context(), session.UserID, accountID, rows)
	if err != nil {
		logger.Error("import_confirm_failed", "user_id", session.UserID, "error", err)
		render.Templ(w, r, http.StatusInternalServerError, templates.ImportErrorFragment("Erro ao importar: "+err.Error()))
		return
	}

	logger.Info("import_confirm_succeeded", "user_id", session.UserID, "inserted", result.Inserted, "duplicates", result.Duplicates, "errors", result.Errors)
	render.Templ(w, r, http.StatusOK, templates.ImportResultFragment(result))
}

func parseCol(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}
