package service

import (
	"errors"
	"math/big"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/siteyonet/siteyonet/internal/domain"
)

type UnitShareInput struct {
	UnitID        uuid.UUID
	OwnerID       uuid.UUID
	LandShare     int
	GrossAreaM2   float64
	ResidentCount int
	IsExempt      bool
	// For mid-period ownership changes
	Transfers []OwnershipTransfer
}

type OwnershipTransfer struct {
	OldUserID        uuid.UUID
	NewUserID        uuid.UUID
	ActiveDaysOld    int
	ActiveDaysNew    int
	TotalDaysInMonth int
}

type AllocatedDebtItem struct {
	UnitID         uuid.UUID
	UserID         uuid.UUID
	PrincipalCents int64
	Notes          string
}

type AccrualEngine struct{}

func NewAccrualEngine() *AccrualEngine {
	return &AccrualEngine{}
}

// DistributeDues performs Largest Remainder Method (Hare-Niemeyer) to ensure cent-exact allocation
func (e *AccrualEngine) DistributeDues(
	totalAmountCents int64,
	distType string,
	units []UnitShareInput,
) ([]AllocatedDebtItem, error) {
	if totalAmountCents <= 0 {
		return nil, errors.New("total amount must be greater than zero")
	}

	// Filter out exempt units from allocation pool
	eligibleUnits := make([]UnitShareInput, 0, len(units))
	for _, u := range units {
		if !u.IsExempt {
			eligibleUnits = append(eligibleUnits, u)
		}
	}

	if len(eligibleUnits) == 0 {
		return nil, errors.New("no eligible units available for distribution")
	}

	type unitCalc struct {
		unit      UnitShareInput
		intShare  int64
		remainder *big.Rat
		index     int
	}
	bases := make([]*big.Rat, len(eligibleUnits))
	totalBase := new(big.Rat)
	for i, u := range eligibleUnits {
		weight := "1"
		switch distType {
		case domain.DistEqual:
		case domain.DistLandShare:
			weight = strconv.Itoa(u.LandShare)
		case domain.DistAreaM2:
			weight = strconv.FormatFloat(u.GrossAreaM2, 'f', 2, 64)
		case domain.DistResidentCount:
			weight = strconv.Itoa(u.ResidentCount)
		default:
			return nil, errors.New("unsupported distribution type")
		}
		base, ok := new(big.Rat).SetString(weight)
		if !ok || base.Sign() < 0 {
			return nil, errors.New("invalid distribution weight")
		}
		bases[i] = base
		totalBase.Add(totalBase, base)
	}
	if totalBase.Sign() <= 0 {
		return nil, errors.New("total distribution basis must be positive")
	}
	calcs := make([]unitCalc, len(eligibleUnits))
	var allocatedSoFar int64
	for i, u := range eligibleUnits {
		quota := new(big.Rat).Mul(bases[i], new(big.Rat).SetInt64(totalAmountCents))
		quota.Quo(quota, totalBase)
		whole := new(big.Int).Quo(quota.Num(), quota.Denom()).Int64()
		rem := new(big.Rat).Sub(quota, new(big.Rat).SetInt64(whole))
		calcs[i] = unitCalc{unit: u, intShare: whole, remainder: rem, index: i}
		allocatedSoFar += whole
	}
	sort.SliceStable(calcs, func(i, j int) bool { return calcs[i].remainder.Cmp(calcs[j].remainder) > 0 })
	for i := int64(0); i < totalAmountCents-allocatedSoFar; i++ {
		calcs[i].intShare++
	}
	// Double check exact sum
	var checkSum int64 = 0
	for _, c := range calcs {
		checkSum += c.intShare
	}
	if checkSum != totalAmountCents {
		return nil, errors.New("cent allocation assertion failed: sum != totalAmountCents")
	}

	// Handle mid-period ownership changes (prorata split)
	result := make([]AllocatedDebtItem, 0, len(calcs)*2)
	for _, c := range calcs {
		if len(c.unit.Transfers) > 0 {
			tr := c.unit.Transfers[0]
			if tr.TotalDaysInMonth <= 0 {
				tr.TotalDaysInMonth = 30
			}
			if tr.ActiveDaysOld < 0 || tr.ActiveDaysNew < 0 || tr.ActiveDaysOld+tr.ActiveDaysNew != tr.TotalDaysInMonth {
				return nil, errors.New("invalid ownership transfer days")
			}
			oldShare := roundRatio(new(big.Int).Mul(big.NewInt(c.intShare), big.NewInt(int64(tr.ActiveDaysOld))), big.NewInt(int64(tr.TotalDaysInMonth)))
			newShare := c.intShare - oldShare

			if oldShare > 0 {
				result = append(result, AllocatedDebtItem{
					UnitID:         c.unit.UnitID,
					UserID:         tr.OldUserID,
					PrincipalCents: oldShare,
					Notes:          "Prorata split (Eski Sahip)",
				})
			}
			if newShare > 0 {
				result = append(result, AllocatedDebtItem{
					UnitID:         c.unit.UnitID,
					UserID:         tr.NewUserID,
					PrincipalCents: newShare,
					Notes:          "Prorata split (Yeni Sahip)",
				})
			}
		} else {
			result = append(result, AllocatedDebtItem{
				UnitID:         c.unit.UnitID,
				UserID:         c.unit.OwnerID,
				PrincipalCents: c.intShare,
				Notes:          "Normal tahakkuk",
			})
		}
	}

	return result, nil
}

// CalculateLateFee computes simple interest for overdue days
func (e *AccrualEngine) CalculateLateFee(principalCents int64, monthlyRatePercent float64, dueDate, calcDate time.Time) int64 {
	if calcDate.Before(dueDate) || principalCents <= 0 || monthlyRatePercent <= 0 {
		return 0
	}

	daysOverdue := int64(calcDate.Sub(dueDate).Hours() / 24)
	if daysOverdue <= 0 {
		return 0
	}

	rate, ok := new(big.Rat).SetString(strconv.FormatFloat(monthlyRatePercent, 'f', -1, 64))
	if !ok {
		return 0
	}
	fee := new(big.Rat).Mul(new(big.Rat).SetInt64(principalCents), rate)
	fee.Mul(fee, new(big.Rat).SetFrac(big.NewInt(daysOverdue), big.NewInt(3000)))
	return roundRatio(fee.Num(), fee.Denom())
}

func roundRatio(num, den *big.Int) int64 {
	n := new(big.Int).Mul(num, big.NewInt(2))
	n.Add(n, den)
	return n.Quo(n, new(big.Int).Mul(den, big.NewInt(2))).Int64()
}
