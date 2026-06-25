package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/csv"
	"errors"
	"finops/internal/store"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type ImportRow struct {
	PostedOn      time.Time
	Description   string
	Amount        float64
	Direction     string
	ExternalFitid string
}

type CSVColumnMap struct {
	DateCol      int
	DateFormat   string // Go format, ex: "02/01/2006"
	DescCol      int
	AmountCol    int
	SignedAmount bool // true: negativo=debit; false: usa DebitCol/CreditCol
	DebitCol     int
	CreditCol    int
}

type ImportResult struct {
	Inserted   int
	Duplicates int
	Errors     int
}

type ImportService interface {
	ParseOFX(r io.Reader) ([]ImportRow, error)
	ParseCSVHeaders(r io.Reader) ([]string, [][]string, error)
	ParseCSVRows(r io.Reader, mapping CSVColumnMap) ([]ImportRow, error)
	StorePreview(rows []ImportRow) string
	LoadPreview(uuid string) ([]ImportRow, bool)
	LastImport(ctx context.Context, ws int64) (time.Time, error)
	ImportRows(ctx context.Context, userID, accountID int64, rows []ImportRow) (ImportResult, error)
}

// --- Temp store ---

type tempEntry struct {
	rows    []ImportRow
	expires time.Time
}

type importTempStore struct {
	mu      sync.RWMutex
	entries map[string]tempEntry
	once    sync.Once
}

func (s *importTempStore) start() {
	s.once.Do(func() {
		go func() {
			for range time.Tick(5 * time.Minute) {
				s.sweep()
			}
		}()
	})
}

func (s *importTempStore) sweep() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, e := range s.entries {
		if now.After(e.expires) {
			delete(s.entries, k)
		}
	}
}

func (s *importTempStore) put(rows []ImportRow) string {
	id := newUUID()
	s.mu.Lock()
	s.entries[id] = tempEntry{rows: rows, expires: time.Now().Add(10 * time.Minute)}
	s.mu.Unlock()
	return id
}

func (s *importTempStore) get(id string) ([]ImportRow, bool) {
	s.mu.RLock()
	e, ok := s.entries[id]
	s.mu.RUnlock()
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	return e.rows, true
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// --- Service ---

type PGImportService struct {
	sqlDB    *sql.DB
	db       *store.Queries
	tmpStore *importTempStore
}

// LastImport implements [ImportService].
func (s *PGImportService) LastImport(ctx context.Context, uid int64) (time.Time, error) {
	ws ,err := s.db.GetWorkSpaceByOwnerUserID(ctx, uid)
	if err != nil {
		return time.Time{}, err
	}
	last, err := s.db.LastImportDate(ctx, ws.ID)
	return last, err
}

func NewPGImportService(sqlDB *sql.DB, q *store.Queries) ImportService {
	ts := &importTempStore{entries: make(map[string]tempEntry)}
	ts.start()
	return &PGImportService{sqlDB: sqlDB, db: q, tmpStore: ts}
}

func (s *PGImportService) StorePreview(rows []ImportRow) string {
	return s.tmpStore.put(rows)
}

func (s *PGImportService) LoadPreview(uuid string) ([]ImportRow, bool) {
	return s.tmpStore.get(uuid)
}

// --- OFX parser ---

var ofxTagRe = regexp.MustCompile(`(?i)<([A-Z0-9.]+)>([^<\n\r]*)`)

func (s *PGImportService) ParseOFX(r io.Reader) ([]ImportRow, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	content := string(raw)

	idx := strings.Index(strings.ToUpper(content), "<OFX>")
	if idx < 0 {
		return nil, errors.New("arquivo OFX inválido: tag <OFX> não encontrada")
	}
	body := content[idx:]

	var rows []ImportRow
	for {
		start := indexCI(body, "<STMTTRN>")
		if start < 0 {
			break
		}
		end := indexCI(body, "</STMTTRN>")
		if end < 0 || end < start {
			break
		}
		block := body[start+len("<STMTTRN>") : end]
		row, err := parseOFXBlock(block)
		if err == nil {
			rows = append(rows, row)
		}
		body = body[end+len("</STMTTRN>"):]
	}

	if len(rows) == 0 {
		return nil, errors.New("nenhuma transação encontrada no arquivo OFX")
	}
	return rows, nil
}

func parseOFXBlock(block string) (ImportRow, error) {
	tags := make(map[string]string)
	for _, match := range ofxTagRe.FindAllStringSubmatch(block, -1) {
		tags[strings.ToUpper(match[1])] = strings.TrimSpace(match[2])
	}

	rawAmt := tags["TRNAMT"]
	if rawAmt == "" {
		return ImportRow{}, errors.New("TRNAMT ausente")
	}
	amt, err := strconv.ParseFloat(strings.ReplaceAll(rawAmt, ",", "."), 64)
	if err != nil {
		return ImportRow{}, fmt.Errorf("TRNAMT inválido: %s", rawAmt)
	}

	rawDate := tags["DTPOSTED"]
	if len(rawDate) < 8 {
		return ImportRow{}, fmt.Errorf("DTPOSTED inválido: %s", rawDate)
	}
	postedOn, err := time.Parse("20060102", rawDate[:8])
	if err != nil {
		return ImportRow{}, fmt.Errorf("DTPOSTED parse falhou: %s", rawDate)
	}

	direction := "credit"
	if amt < 0 {
		direction = "debit"
	}
	trnType := strings.ToUpper(tags["TRNTYPE"])
	if trnType == "DEBIT" || trnType == "CHECK" || trnType == "ATM" || trnType == "FEE" {
		direction = "debit"
	}

	desc := tags["MEMO"]
	if desc == "" {
		desc = tags["NAME"]
	}
	if desc == "" {
		desc = trnType
	}

	return ImportRow{
		PostedOn:      postedOn,
		Description:   desc,
		Amount:        math.Abs(amt),
		Direction:     direction,
		ExternalFitid: strings.TrimSpace(tags["FITID"]),
	}, nil
}

func indexCI(s, sub string) int {
	return strings.Index(strings.ToUpper(s), strings.ToUpper(sub))
}

// --- CSV parser ---

func (s *PGImportService) ParseCSVHeaders(r io.Reader) ([]string, [][]string, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	all, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("erro ao ler CSV: %w", err)
	}
	if len(all) == 0 {
		return nil, nil, errors.New("arquivo CSV vazio")
	}

	headers := all[0]
	end := len(all)
	if end > 6 {
		end = 6 // header + 5 linhas de amostra
	}
	return headers, all[1:end], nil
}

func (s *PGImportService) ParseCSVRows(r io.Reader, m CSVColumnMap) ([]ImportRow, error) {
	if m.DateFormat == "" {
		m.DateFormat = "02/01/2006"
	}

	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	all, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("erro ao ler CSV: %w", err)
	}
	if len(all) <= 1 {
		return nil, errors.New("CSV sem dados")
	}

	rows := all[1:] // pula header
	var result []ImportRow

	for i, row := range rows {
		if !safeIdx(row, m.DateCol) || !safeIdx(row, m.DescCol) {
			continue
		}

		postedOn, err := time.Parse(m.DateFormat, strings.TrimSpace(row[m.DateCol]))
		if err != nil {
			continue
		}

		desc := strings.TrimSpace(row[m.DescCol])
		if desc == "" {
			continue
		}

		var amount float64
		var direction string

		if m.SignedAmount {
			if !safeIdx(row, m.AmountCol) {
				continue
			}
			raw := normalizeCSVAmount(row[m.AmountCol])
			amount, err = strconv.ParseFloat(raw, 64)
			if err != nil {
				continue
			}
			if amount < 0 {
				direction = "debit"
				amount = math.Abs(amount)
			} else {
				direction = "credit"
			}
		} else {
			debit, credit := 0.0, 0.0
			if safeIdx(row, m.DebitCol) {
				debit, _ = strconv.ParseFloat(normalizeCSVAmount(row[m.DebitCol]), 64)
			}
			if safeIdx(row, m.CreditCol) {
				credit, _ = strconv.ParseFloat(normalizeCSVAmount(row[m.CreditCol]), 64)
			}
			if debit > 0 {
				amount, direction = debit, "debit"
			} else if credit > 0 {
				amount, direction = credit, "credit"
			} else {
				_ = i
				continue
			}
		}

		if amount <= 0 {
			continue
		}

		result = append(result, ImportRow{
			PostedOn:    postedOn,
			Description: desc,
			Amount:      amount,
			Direction:   direction,
		})
	}

	return result, nil
}

func safeIdx(row []string, idx int) bool {
	return idx >= 0 && idx < len(row)
}

func normalizeCSVAmount(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "R$", "")
	s = strings.ReplaceAll(s, " ", "")
	// formato BR: 1.234,56 → 1234.56
	if strings.Count(s, ".") >= 1 && strings.Count(s, ",") == 1 {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	} else if strings.Count(s, ",") == 1 && !strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ",", ".")
	}
	return s
}

// --- ImportRows ---

func (s *PGImportService) ImportRows(ctx context.Context, userID, accountID int64, rows []ImportRow) (ImportResult, error) {
	ws, err := s.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return ImportResult{}, fmt.Errorf("workspace não encontrado: %w", err)
	}

	account, err := s.db.GetAccountByWorkspaceAndID(ctx, store.GetAccountByWorkspaceAndIDParams{
		WorkspaceID: ws.ID,
		ID:          accountID,
	})
	if err != nil {
		return ImportResult{}, fmt.Errorf("conta não encontrada: %w", err)
	}

	

	var result ImportResult
	for _, row := range rows {
		fitid := sql.NullString{}
		if row.ExternalFitid != "" {
			fitid = sql.NullString{String: row.ExternalFitid, Valid: true}
		}

		_, err := s.db.CreateTransaction(ctx, store.CreateTransactionParams{
			WorkspaceID:     ws.ID,
			AccountID:       account.ID,
			CategoryID:      sql.NullInt64{},
			PostedOn:        row.PostedOn,
			Description:     row.Description,
			Amount:          formatNumeric(row.Amount),
			Direction:       row.Direction,
			Currency:        account.Currency,
			TransferGroupID: sql.NullInt64{},
			ExternalFitid:   fitid,
			Source:          "import",
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				result.Duplicates++
				continue
			}
			result.Errors++
			continue
		}
		result.Inserted++
	}

	if result.Inserted > 0 {
		if err = s.db.InsertImportDate(ctx, ws.ID); err != nil {
			return ImportResult{}, err
		}
	}

	return result, nil
}
