package service

import (
	"errors"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/siteyonet/siteyonet/internal/domain"
)

type PaymentService struct {
	accrualEngine *AccrualEngine
}

func NewPaymentService(accrualEngine *AccrualEngine) *PaymentService {
	return &PaymentService{
		accrualEngine: accrualEngine,
	}
}

type AllocationResult struct {
	AllocatedDebts   []AllocatedDebtDetail
	UnallocatedCents int64
	TotalPaidCents   int64
}

type AllocatedDebtDetail struct {
	DebtID         uuid.UUID
	PrincipalPaid  int64
	InterestPaid   int64
	RemainingCents int64
	NewStatus      string
}

// AllocatePayment implements the waterfall allocation:
// 1. Interest first
// 2. Principal next
// 3. Oldest debt first (FIFO)
// Any surplus is stored as unallocated credit balance
func (s *PaymentService) AllocatePayment(
	paymentAmountCents int64,
	debts []*domain.Debt,
	monthlyLateFeeRate float64,
	paymentDate time.Time,
) (*AllocationResult, error) {
	if paymentAmountCents <= 0 {
		return nil, errors.New("payment amount must be greater than zero")
	}

	// Filter unpaid or partially paid debts
	activeDebts := make([]*domain.Debt, 0)
	for _, d := range debts {
		if d.Status == domain.DebtStatusUnpaid || d.Status == domain.DebtStatusPartial {
			// Calculate accrued late fee as of payment date
			unpaidPrincipal := d.PrincipalCents - d.PaidPrincipalCents
			d.RemainingCents = unpaidPrincipal
			d.CalculatedFeeCents = s.accrualEngine.CalculateLateFee(unpaidPrincipal, monthlyLateFeeRate, d.DueDate, paymentDate)
			activeDebts = append(activeDebts, d)
		}
	}

	// Sort FIFO: oldest due date first
	sort.Slice(activeDebts, func(i, j int) bool {
		return activeDebts[i].DueDate.Before(activeDebts[j].DueDate)
	})

	remainingPayment := paymentAmountCents
	result := &AllocationResult{
		AllocatedDebts: make([]AllocatedDebtDetail, 0),
		TotalPaidCents: paymentAmountCents,
	}

	// Waterfall phase 1 & 2 for each debt in FIFO order
	for _, debt := range activeDebts {
		if remainingPayment <= 0 {
			break
		}

		interestToPay := debt.CalculatedFeeCents - debt.PaidInterestCents
		if interestToPay < 0 {
			interestToPay = 0
		}
		principalToPay := debt.PrincipalCents - debt.PaidPrincipalCents

		var allocatedInterest int64 = 0
		var allocatedPrincipal int64 = 0

		// Step 1: Pay interest first
		if interestToPay > 0 {
			if remainingPayment >= interestToPay {
				allocatedInterest = interestToPay
				remainingPayment -= interestToPay
			} else {
				allocatedInterest = remainingPayment
				remainingPayment = 0
			}
		}

		// Step 2: Pay principal next
		if remainingPayment > 0 && principalToPay > 0 {
			if remainingPayment >= principalToPay {
				allocatedPrincipal = principalToPay
				remainingPayment -= principalToPay
			} else {
				allocatedPrincipal = remainingPayment
				remainingPayment = 0
			}
		}

		newPaidPrincipal := debt.PaidPrincipalCents + allocatedPrincipal
		newPaidInterest := debt.PaidInterestCents + allocatedInterest
		remainingPrincipal := debt.PrincipalCents - newPaidPrincipal

		newStatus := domain.DebtStatusUnpaid
		if remainingPrincipal <= 0 {
			newStatus = domain.DebtStatusPaid
		} else if newPaidPrincipal > 0 || newPaidInterest > 0 {
			newStatus = domain.DebtStatusPartial
		}

		result.AllocatedDebts = append(result.AllocatedDebts, AllocatedDebtDetail{
			DebtID:         debt.ID,
			PrincipalPaid:  allocatedPrincipal,
			InterestPaid:   allocatedInterest,
			RemainingCents: remainingPrincipal,
			NewStatus:      newStatus,
		})
	}

	// Step 3: Any surplus amount is saved as unallocated advance credit
	result.UnallocatedCents = remainingPayment
	return result, nil
}

// FuzzyMatchBankStatement extracts unit number and block from raw bank transfer description
// Uses regex pattern matching + Levenshtein distance for fuzzy matching
type MatchCandidate struct {
	UnitID     uuid.UUID `json:"unit_id"`
	BlockName  string    `json:"block_name"`
	UnitNumber string    `json:"unit_number"`
	Score      float64   `json:"score"`
}

func (s *PaymentService) FuzzyMatchUnit(rawDescription string, allUnits []domain.Unit) (*MatchCandidate, error) {
	if len(allUnits) == 0 {
		return nil, errors.New("no units provided for matching")
	}

	cleaned := strings.ToUpper(rawDescription)
	// Replace Turkish specific chars for uniform matching
	cleaned = strings.ReplaceAll(cleaned, "İ", "I")
	cleaned = strings.ReplaceAll(cleaned, "Ş", "S")
	cleaned = strings.ReplaceAll(cleaned, "Ğ", "G")
	cleaned = strings.ReplaceAll(cleaned, "Ü", "U")
	cleaned = strings.ReplaceAll(cleaned, "Ö", "O")
	cleaned = strings.ReplaceAll(cleaned, "Ç", "C")

	// Regex heuristics:
	// 1. "DAIRE: 12", "D: 12", "D12", "NO: 12", "NO 12"
	// 2. "A BLOK 12", "B BLOK NO: 5"
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:DAIRE|D|NO|KAPI)[\s\.:\-_]*([0-9]{1,4})`),
		regexp.MustCompile(`([A-Z0-9]+)\s*BLOK[\s\.:\-_]*(?:D|DAIRE|NO)?[\s\.:\-_]*([0-9]{1,4})`),
		regexp.MustCompile(`([A-Z])[\s\-_]*([0-9]{1,4})`),
	}

	var extractedBlock string
	var extractedNumber string

	for _, p := range patterns {
		matches := p.FindStringSubmatch(cleaned)
		if len(matches) == 2 {
			extractedNumber = matches[1]
			break
		} else if len(matches) == 3 {
			extractedBlock = matches[1]
			extractedNumber = matches[2]
			break
		}
	}

	var bestCandidate *MatchCandidate
	highestScore := 0.0
	secondScore := 0.0

	for _, unit := range allUnits {
		score := 0.0
		unitNum := strings.TrimSpace(unit.UnitNumber)
		blockName := strings.ToUpper(strings.TrimSpace(unit.BlockName))

		if extractedNumber != "" && extractedNumber != unitNum {
			continue
		}
		blockParts := strings.Fields(blockName)
		if extractedBlock != "" && (len(blockParts) == 0 || blockParts[0] != extractedBlock) {
			continue
		}
		// Exact unit number match
		if extractedNumber != "" && extractedNumber == unitNum {
			score += 0.60
		} else if strings.Contains(cleaned, " "+unitNum+" ") || strings.Contains(cleaned, "D"+unitNum) || strings.Contains(cleaned, "NO"+unitNum) {
			score += 0.50
		} else {
			// Levenshtein similarity on tokens
			sim := stringSimilarity(cleaned, unitNum)
			score += sim * 0.30
		}

		// Block match
		if extractedBlock != "" && len(blockParts) > 0 && blockParts[0] == extractedBlock {
			score += 0.40
		} else if strings.Contains(cleaned, blockName) {
			score += 0.35
		}

		// Resident name match
		if unit.OwnerName != "" {
			ownerClean := strings.ToUpper(unit.OwnerName)
			ownerClean = strings.ReplaceAll(ownerClean, "İ", "I")
			tokens := strings.Fields(ownerClean)
			for _, t := range tokens {
				if len(t) > 2 && strings.Contains(cleaned, t) {
					score += 0.20
					break
				}
			}
		}

		if score > highestScore {
			secondScore = highestScore
			highestScore = score
			bestCandidate = &MatchCandidate{
				UnitID:     unit.ID,
				BlockName:  unit.BlockName,
				UnitNumber: unit.UnitNumber,
				Score:      math.Min(score, 1.0),
			}
		} else if score > secondScore {
			secondScore = score
		}
	}

	if bestCandidate == nil || highestScore < 0.50 || highestScore-secondScore < 0.10 {
		return nil, errors.New("no confident unit match found")
	}

	return bestCandidate, nil
}

// Levenshtein distance & similarity
func stringSimilarity(s1, s2 string) float64 {
	d := levenshtein([]rune(s1), []rune(s2))
	maxLen := math.Max(float64(len(s1)), float64(len(s2)))
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - (float64(d) / maxLen)
}

func levenshtein(a, b []rune) int {
	la := len(a)
	lb := len(b)
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			d[i][j] = min(
				d[i-1][j]+1,
				d[i][j-1]+1,
				d[i-1][j-1]+cost,
			)
		}
	}
	return d[la][lb]
}

func min(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// CreateRestructurePlan builds a structured installment plan for overdue debts
type RestructurePlan struct {
	TotalAmountCents int64
	Installments     []RestructureInstallment
}

type RestructureInstallment struct {
	InstallmentNumber int
	DueDate           time.Time
	AmountCents       int64
}

func (s *PaymentService) CreateRestructurePlan(
	totalOverdueCents int64,
	installmentCount int,
	startDate time.Time,
) (*RestructurePlan, error) {
	if totalOverdueCents <= 0 || installmentCount <= 0 {
		return nil, errors.New("invalid amount or installment count")
	}

	baseCents := totalOverdueCents / int64(installmentCount)
	remainder := totalOverdueCents % int64(installmentCount)

	plan := &RestructurePlan{
		TotalAmountCents: totalOverdueCents,
		Installments:     make([]RestructureInstallment, installmentCount),
	}

	for i := 0; i < installmentCount; i++ {
		amount := baseCents
		if int64(i) < remainder {
			amount += 1 // distribute remaining cents
		}

		dueDate := startDate.AddDate(0, i+1, 0)
		plan.Installments[i] = RestructureInstallment{
			InstallmentNumber: i + 1,
			DueDate:           dueDate,
			AmountCents:       amount,
		}
	}

	return plan, nil
}

// CalculateDelayInterest calculates simple delay interest for given principal, monthly rate, and days overdue
func (s *PaymentService) CalculateDelayInterest(principalCents int64, monthlyLateFeeRate float64, daysOverdue int) int64 {
	dueDate := time.Now().AddDate(0, 0, -daysOverdue)
	return s.accrualEngine.CalculateLateFee(principalCents, monthlyLateFeeRate, dueDate, time.Now())
}
