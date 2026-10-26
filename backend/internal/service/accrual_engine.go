package service

import (
	"errors"
	"math"
	"sort"
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
	OldUserID  uuid.UUID
	NewUserID  uuid.UUID
	ActiveDaysOld int
	ActiveDaysNew int
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

	// Calculate total base according to distribution type
	var totalBase float64
	for _, u := range eligibleUnits {
		switch distType {
		case domain.DistEqual:
			totalBase += 1.0
		case domain.DistLandShare:
			totalBase += float64(u.LandShare)
		case domain.DistAreaM2:
			totalBase += u.GrossAreaM2
		case domain.DistResidentCount:
			totalBase += float64(u.ResidentCount)
		default:
			return nil, errors.New("unsupported distribution type")
		}
	}

	if totalBase <= 0 {
		return nil, errors.New("total distribution basis must be greater than zero")
	}

	type unitCalc struct {
		unit       UnitShareInput
		exactQuota float64
		intShare   int64
		remainder  float64
		index      int
	}

	calcs := make([]unitCalc, len(eligibleUnits))
	var allocatedSoFar int64 = 0

	for i, u := range eligibleUnits {
		var unitBase float64
		switch distType {
		case domain.DistEqual:
			unitBase = 1.0
		case domain.DistLandShare:
			unitBase = float64(u.LandShare)
		case domain.DistAreaM2:
			unitBase = u.GrossAreaM2
		case domain.DistResidentCount:
			unitBase = float64(u.ResidentCount)
		}

		quota := (unitBase / totalBase) * float64(totalAmountCents)
		floorQuota := int64(math.Floor(quota))
		rem := quota - float64(floorQuota)

		calcs[i] = unitCalc{
			unit:       u,
			exactQuota: quota,
			intShare:   floorQuota,
			remainder:  rem,
			index:      i,
		}
		allocatedSoFar += floorQuota
	}

	// Largest Remainder Method: distribute remaining cents based on highest remainder
	remainingCents := totalAmountCents - allocatedSoFar
	if remainingCents > 0 {
		// Sort by remainder descending, tie-break by unit index
		sort.Slice(calcs, func(i, j int) bool {
			if calcs[i].remainder == calcs[j].remainder {
				return calcs[i].index < calcs[j].index
			}
			return calcs[i].remainder > calcs[j].remainder
		})

		for i := 0; i < int(remainingCents); i++ {
			calcs[i%len(calcs)].intShare += 1
		}
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
			oldShare := int64(math.Round(float64(c.intShare) * float64(tr.ActiveDaysOld) / float64(tr.TotalDaysInMonth)))
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

	// Daily rate based on 30-day month convention: (monthlyRate / 100) / 30
	dailyRate := (monthlyRatePercent / 100.0) / 30.0
	fee := float64(principalCents) * dailyRate * float64(daysOverdue)

	return int64(math.Round(fee))
}
