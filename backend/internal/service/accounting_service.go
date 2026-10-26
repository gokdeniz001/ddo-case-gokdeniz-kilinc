package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/siteyonet/siteyonet/internal/domain"
)

type AccountingService struct{}

func NewAccountingService() *AccountingService {
	return &AccountingService{}
}

type JournalEntryInput struct {
	SiteID      uuid.UUID
	PeriodID    uuid.UUID
	EntryDate   time.Time
	Description string
	Lines       []JournalLineInput
}

type JournalLineInput struct {
	AccountID   uuid.UUID
	DebitCents  int64
	CreditCents int64
	Description string
}

// ValidateDoubleEntry verifies that Sum(Debit) == Sum(Credit) and neither side is 0
func (s *AccountingService) ValidateDoubleEntry(lines []JournalLineInput) error {
	if len(lines) < 2 {
		return errors.New("a double-entry journal entry must have at least 2 lines")
	}

	var totalDebit int64 = 0
	var totalCredit int64 = 0

	for _, l := range lines {
		if l.DebitCents < 0 || l.CreditCents < 0 {
			return errors.New("debit and credit amounts cannot be negative")
		}
		if l.DebitCents > 0 && l.CreditCents > 0 {
			return errors.New("a single line cannot contain both debit and credit")
		}
		if l.DebitCents == 0 && l.CreditCents == 0 {
			return errors.New("journal line must have either debit or credit amount")
		}
		totalDebit += l.DebitCents
		totalCredit += l.CreditCents
	}

	if totalDebit == 0 {
		return errors.New("journal entry total debit must be greater than zero")
	}

	if totalDebit != totalCredit {
		return fmt.Errorf("double-entry unbalanced: total debit (%d cents) != total credit (%d cents)", totalDebit, totalCredit)
	}

	return nil
}

// CreateReversalEntry creates a reversing journal entry (swapping debit and credit)
func (s *AccountingService) CreateReversalEntry(original domain.JournalEntry, reversalDate time.Time, reason string) (*JournalEntryInput, error) {
	if len(original.Lines) == 0 {
		return nil, errors.New("cannot reverse journal entry without lines")
	}

	reversalLines := make([]JournalLineInput, len(original.Lines))
	for i, line := range original.Lines {
		// Swap debits and credits
		reversalLines[i] = JournalLineInput{
			AccountID:   line.AccountID,
			DebitCents:  line.CreditCents,
			CreditCents: line.DebitCents,
			Description: fmt.Sprintf("Reversal of entry #%d: %s", original.EntryNumber, line.Description),
		}
	}

	return &JournalEntryInput{
		SiteID:      original.SiteID,
		PeriodID:    original.PeriodID,
		EntryDate:   reversalDate,
		Description: fmt.Sprintf("Ters Kayıt (Reversal) - Fiş #%d: %s", original.EntryNumber, reason),
		Lines:       reversalLines,
	}, nil
}

// BudgetComparisonItem reports variance between budgeted and actual expenses
type BudgetComparisonItem struct {
	Category        string  `json:"category"`
	BudgetCents     int64   `json:"budget_cents"`
	ActualCents     int64   `json:"actual_cents"`
	VarianceCents   int64   `json:"variance_cents"` // Budget - Actual
	VariancePercent float64 `json:"variance_percent"`
}

func (s *AccountingService) CalculateBudgetComparison(budgets map[string]int64, actuals map[string]int64) []BudgetComparisonItem {
	allCats := make(map[string]bool)
	for c := range budgets {
		allCats[c] = true
	}
	for c := range actuals {
		allCats[c] = true
	}

	results := make([]BudgetComparisonItem, 0, len(allCats))
	for cat := range allCats {
		b := budgets[cat]
		a := actuals[cat]
		v := b - a
		pct := 0.0
		if b > 0 {
			pct = (float64(a) / float64(b)) * 100.0
		}
		results = append(results, BudgetComparisonItem{
			Category:        cat,
			BudgetCents:     b,
			ActualCents:     a,
			VarianceCents:   v,
			VariancePercent: pct,
		})
	}
	return results
}
