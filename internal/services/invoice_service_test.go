package service

import (
	"testing"
	"time"
)

func TestDedupProposalRemovesExistingCommitments(t *testing.T) {
	jul := month(2026, 7)
	existing := []Commitment{
		{Name: "Netflix", Kind: CommitmentSubscription, MonthlyValue: 71.83, StartMonth: jul},
	}
	prop := InvoiceProposal{
		Commitments: []CommitmentProposal{
			{Name: "Netflix", Kind: CommitmentSubscription, MonthlyValue: 71.83, StartMonth: jul}, // já existe
			{Name: "KaBuM (10x)", Kind: CommitmentInstallment, MonthlyValue: 718.38, StartMonth: jul, EndMonth: mp(month(2027, 4))},
		},
	}

	out := dedupProposal(prop, existing)
	if len(out.Commitments) != 1 {
		t.Fatalf("commitments após dedup: got %d, want 1", len(out.Commitments))
	}
	if out.Commitments[0].Name != "KaBuM (10x)" {
		t.Errorf("commitment restante: got %q, want KaBuM", out.Commitments[0].Name)
	}
}

func TestDedupProposalDedupsTransactionsInternally(t *testing.T) {
	day := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	prop := InvoiceProposal{
		Transactions: []ImportRow{
			{PostedOn: day, Description: "Mercado X", Amount: 100, Direction: "debit"},
			{PostedOn: day, Description: "Mercado X", Amount: 100, Direction: "debit"}, // duplicata exata
			{PostedOn: day, Description: "Posto Y", Amount: 200, Direction: "debit"},
		},
	}
	out := dedupProposal(prop, nil)
	if len(out.Transactions) != 2 {
		t.Fatalf("transações após dedup: got %d, want 2", len(out.Transactions))
	}
	// FITID sintético preenchido para reusar o índice único no import.
	for _, tx := range out.Transactions {
		if tx.ExternalFitid == "" {
			t.Error("transação sem FITID sintético após dedup")
		}
	}
}

// A mesma fatura importada duas vezes: na segunda, todos os commitments já
// existem e as transações têm o mesmo FITID sintético — nada novo entra.
func TestDedupProposalSecondImportIsEmpty(t *testing.T) {
	jul := month(2026, 7)
	day := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	firstImport := InvoiceProposal{
		Transactions: []ImportRow{{PostedOn: day, Description: "Mercado X", Amount: 100, Direction: "debit"}},
		Commitments:  []CommitmentProposal{{Name: "Netflix", Kind: CommitmentSubscription, MonthlyValue: 71.83, StartMonth: jul}},
	}

	// Simula o estado após a 1ª importação já persistida.
	existing := []Commitment{{Name: "Netflix", Kind: CommitmentSubscription, MonthlyValue: 71.83, StartMonth: jul}}

	second := dedupProposal(firstImport, existing)
	if len(second.Commitments) != 0 {
		t.Errorf("2ª importação não deve recriar commitments: got %d", len(second.Commitments))
	}
	// As transações passam pelo dedup interno (não duplicam entre si), mas o
	// backstop real contra re-importação é o FITID sintético no índice único —
	// que aqui garantimos estar preenchido de forma estável.
	want := syntheticFitid(firstImport.Transactions[0])
	if len(second.Transactions) != 1 || second.Transactions[0].ExternalFitid != want {
		t.Errorf("FITID sintético instável entre importações: got %+v", second.Transactions)
	}
}
