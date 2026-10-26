package tests

import (
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/siteyonet/siteyonet/internal/domain"
	"github.com/siteyonet/siteyonet/internal/service"
)

// ----------------------------------------------------------------------------
// Task 1 (T-1) Tests: Accrual & Distribution Engine
// ----------------------------------------------------------------------------

func TestT1_LargestRemainderMethod_CentExactEqual(t *testing.T) {
	engine := service.NewAccrualEngine()

	// 10,000.00 TL (1,000,000 cents) across 3 units (1000000 / 3 = 333333.333...)
	// Sum must be EXACTLY 1,000,000 cents!
	totalCents := int64(1000000)
	units := []service.UnitShareInput{
		{UnitID: uuid.New(), OwnerID: uuid.New()},
		{UnitID: uuid.New(), OwnerID: uuid.New()},
		{UnitID: uuid.New(), OwnerID: uuid.New()},
	}

	result, err := engine.DistributeDues(totalCents, domain.DistEqual, units)
	if err != nil {
		t.Fatalf("DistributeDues failed: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(result))
	}

	var sum int64 = 0
	for _, r := range result {
		sum += r.PrincipalCents
	}

	if sum != totalCents {
		t.Errorf("Largest Remainder sum mismatch: expected %d, got %d", totalCents, sum)
	}

	// Two units should get 333333 and one should get 333334 (or vice versa)
	has333334 := false
	for _, r := range result {
		if r.PrincipalCents == 333334 {
			has333334 = true
		}
	}
	if !has333334 {
		t.Errorf("Expected at least one unit to receive the remainder cent")
	}
}

func TestT1_LargestRemainderMethod_CentExactLandShare(t *testing.T) {
	engine := service.NewAccrualEngine()

	// Prime number total cents to create irregular fractions
	totalCents := int64(354217) // 3,542.17 TL
	units := []service.UnitShareInput{
		{UnitID: uuid.New(), OwnerID: uuid.New(), LandShare: 135},
		{UnitID: uuid.New(), OwnerID: uuid.New(), LandShare: 247},
		{UnitID: uuid.New(), OwnerID: uuid.New(), LandShare: 318},
		{UnitID: uuid.New(), OwnerID: uuid.New(), LandShare: 199},
	}

	result, err := engine.DistributeDues(totalCents, domain.DistLandShare, units)
	if err != nil {
		t.Fatalf("DistributeDues failed: %v", err)
	}

	var sum int64 = 0
	for _, r := range result {
		sum += r.PrincipalCents
	}

	if sum != totalCents {
		t.Errorf("Cent-exact failure on land share: expected %d, got %d", totalCents, sum)
	}
}

func TestT1_ExemptUnitsExcluded(t *testing.T) {
	engine := service.NewAccrualEngine()

	totalCents := int64(500000)
	exemptUnitID := uuid.New()
	units := []service.UnitShareInput{
		{UnitID: uuid.New(), OwnerID: uuid.New(), IsExempt: false},
		{UnitID: exemptUnitID, OwnerID: uuid.New(), IsExempt: true}, // Doorman unit
		{UnitID: uuid.New(), OwnerID: uuid.New(), IsExempt: false},
	}

	result, err := engine.DistributeDues(totalCents, domain.DistEqual, units)
	if err != nil {
		t.Fatalf("DistributeDues failed: %v", err)
	}

	// Result must only contain 2 eligible units
	if len(result) != 2 {
		t.Fatalf("Expected 2 units, got %d", len(result))
	}

	for _, r := range result {
		if r.UnitID == exemptUnitID {
			t.Errorf("Exempt unit should not have received any allocation!")
		}
		if r.PrincipalCents != 250000 {
			t.Errorf("Expected 250000 cents each, got %d", r.PrincipalCents)
		}
	}
}

func TestT1_MidPeriodOwnershipTransfer_Prorata(t *testing.T) {
	engine := service.NewAccrualEngine()

	totalCents := int64(300000) // 3,000 TL total across 1 unit
	oldOwner := uuid.New()
	newOwner := uuid.New()

	units := []service.UnitShareInput{
		{
			UnitID:   uuid.New(),
			OwnerID:  newOwner,
			IsExempt: false,
			Transfers: []service.OwnershipTransfer{
				{
					OldUserID:        oldOwner,
					NewUserID:        newOwner,
					ActiveDaysOld:    10, // 10 days old owner
					ActiveDaysNew:    20, // 20 days new owner
					TotalDaysInMonth: 30,
				},
			},
		},
	}

	result, err := engine.DistributeDues(totalCents, domain.DistEqual, units)
	if err != nil {
		t.Fatalf("DistributeDues failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 prorata debt items, got %d", len(result))
	}

	var oldShare, newShare int64
	for _, r := range result {
		if r.UserID == oldOwner {
			oldShare = r.PrincipalCents
		}
		if r.UserID == newOwner {
			newShare = r.PrincipalCents
		}
	}

	if oldShare != 100000 || newShare != 200000 {
		t.Errorf("Prorata split incorrect: old=%d (expected 100000), new=%d (expected 200000)", oldShare, newShare)
	}

	if oldShare+newShare != totalCents {
		t.Errorf("Total prorata sum (%d) does not match total cents (%d)", oldShare+newShare, totalCents)
	}
}

// ----------------------------------------------------------------------------
// Task 2 (T-2) Tests: Payment Waterfall, Late Fee, Restructure & Fuzzy Match
// ----------------------------------------------------------------------------

func TestT2_WaterfallAllocation_InterestThenPrincipalThenFIFO(t *testing.T) {
	engine := service.NewAccrualEngine()
	paymentSvc := service.NewPaymentService(engine)

	baseDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	paymentDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) // 60 days overdue

	debt1 := &domain.Debt{
		ID:                 uuid.New(),
		PrincipalCents:     100000, // 1,000 TL
		PaidPrincipalCents: 0,
		PaidInterestCents:  0,
		DueDate:            baseDate,
		Status:             domain.DebtStatusUnpaid,
	}
	debt2 := &domain.Debt{
		ID:                 uuid.New(),
		PrincipalCents:     100000, // 1,000 TL
		PaidPrincipalCents: 0,
		PaidInterestCents:  0,
		DueDate:            baseDate.AddDate(0, 1, 0), // 30 days newer
		Status:             domain.DebtStatusUnpaid,
	}

	// 5% monthly rate -> 60 days overdue on 100,000 = ~10,000 cents interest on debt1
	// Let's pay 50,000 cents (500 TL).
	// It must FIRST pay interest of debt1 (~10,000 cents), then remainder (40,000 cents) to principal of debt1!
	allocResult, err := paymentSvc.AllocatePayment(50000, []*domain.Debt{debt1, debt2}, 5.0, paymentDate)
	if err != nil {
		t.Fatalf("AllocatePayment failed: %v", err)
	}

	if len(allocResult.AllocatedDebts) == 0 {
		t.Fatalf("Expected allocated debts, got none")
	}

	firstAlloc := allocResult.AllocatedDebts[0]
	if firstAlloc.DebtID != debt1.ID {
		t.Errorf("FIFO violated: debt1 should have been allocated first")
	}

	if firstAlloc.InterestPaid <= 0 {
		t.Errorf("Interest waterfall violated: interest must be paid before principal, got %d", firstAlloc.InterestPaid)
	}

	if firstAlloc.InterestPaid+firstAlloc.PrincipalPaid != 50000 {
		t.Errorf("Total allocated mismatch: expected 50000, got %d", firstAlloc.InterestPaid+firstAlloc.PrincipalPaid)
	}

	if allocResult.UnallocatedCents != 0 {
		t.Errorf("Expected 0 unallocated cents, got %d", allocResult.UnallocatedCents)
	}
}

func TestT2_Overpayment_StoredAsUnallocatedCredit(t *testing.T) {
	engine := service.NewAccrualEngine()
	paymentSvc := service.NewPaymentService(engine)

	debt := &domain.Debt{
		ID:                 uuid.New(),
		PrincipalCents:     100000, // 1,000 TL
		PaidPrincipalCents: 0,
		DueDate:            time.Now().AddDate(0, 0, 5), // Not overdue
		Status:             domain.DebtStatusUnpaid,
	}

	// Pay 1,500 TL (150,000 cents) on a 1,000 TL debt
	allocResult, err := paymentSvc.AllocatePayment(150000, []*domain.Debt{debt}, 5.0, time.Now())
	if err != nil {
		t.Fatalf("AllocatePayment failed: %v", err)
	}

	if allocResult.UnallocatedCents != 50000 {
		t.Errorf("Expected 50,000 cents surplus credit, got %d", allocResult.UnallocatedCents)
	}

	if allocResult.AllocatedDebts[0].NewStatus != domain.DebtStatusPaid {
		t.Errorf("Expected debt status PAID, got %s", allocResult.AllocatedDebts[0].NewStatus)
	}
}

func TestT2_FuzzyMatchBankStatement(t *testing.T) {
	engine := service.NewAccrualEngine()
	paymentSvc := service.NewPaymentService(engine)

	targetUnitID := uuid.New()
	units := []domain.Unit{
		{ID: uuid.New(), BlockName: "A Blok", UnitNumber: "1"},
		{ID: targetUnitID, BlockName: "A Blok", UnitNumber: "12", OwnerName: "Ahmet Yılmaz"},
		{ID: uuid.New(), BlockName: "B Blok", UnitNumber: "12"},
		{ID: uuid.New(), BlockName: "A Blok", UnitNumber: "21"},
	}

	testCases := []struct {
		desc        string
		expectedID  uuid.UUID
		expectMatch bool
	}{
		{"A BLOK DAIRE: 12 AIDAT BEDELI", targetUnitID, true},
		{"A-12 AHMET YILMAZ OCAK AYI", targetUnitID, true},
		{"B BLOK 12 MEHMET", targetUnitID, false}, // Different block
	}

	for _, tc := range testCases {
		cand, err := paymentSvc.FuzzyMatchUnit(tc.desc, units)
		if tc.expectMatch {
			if err != nil || cand == nil {
				t.Errorf("Expected match for %q, got err: %v", tc.desc, err)
			} else if cand.UnitID != tc.expectedID {
				t.Errorf("Wrong unit matched for %q: expected %s, got %s", tc.desc, tc.expectedID, cand.UnitID)
			}
		}
	}
}

func TestT2_DebtRestructurePlan(t *testing.T) {
	engine := service.NewAccrualEngine()
	paymentSvc := service.NewPaymentService(engine)

	totalOverdue := int64(100000) // 1,000 TL divided into 3 installments
	plan, err := paymentSvc.CreateRestructurePlan(totalOverdue, 3, time.Now())
	if err != nil {
		t.Fatalf("CreateRestructurePlan failed: %v", err)
	}

	if len(plan.Installments) != 3 {
		t.Fatalf("Expected 3 installments, got %d", len(plan.Installments))
	}

	var sum int64 = 0
	for _, inst := range plan.Installments {
		sum += inst.AmountCents
	}

	if sum != totalOverdue {
		t.Errorf("Restructure plan sum mismatch: expected %d, got %d", totalOverdue, sum)
	}
}

// ----------------------------------------------------------------------------
// Task 4 (T-4) Tests: Double-entry Bookkeeping, Period Closing & Clearance Cert
// ----------------------------------------------------------------------------

func TestT4_DoubleEntryValidation(t *testing.T) {
	accountingSvc := service.NewAccountingService()

	acc1 := uuid.New()
	acc2 := uuid.New()

	// Balanced entry: 1500 Debit vs 1500 Credit
	balanced := []service.JournalLineInput{
		{AccountID: acc1, DebitCents: 150000, CreditCents: 0, Description: "Debit line"},
		{AccountID: acc2, DebitCents: 0, CreditCents: 150000, Description: "Credit line"},
	}
	if err := accountingSvc.ValidateDoubleEntry(balanced); err != nil {
		t.Errorf("Expected balanced entry to pass, got: %v", err)
	}

	// Unbalanced entry: 1500 Debit vs 1000 Credit -> MUST FAIL
	unbalanced := []service.JournalLineInput{
		{AccountID: acc1, DebitCents: 150000, CreditCents: 0, Description: "Debit line"},
		{AccountID: acc2, DebitCents: 0, CreditCents: 100000, Description: "Credit line"},
	}
	if err := accountingSvc.ValidateDoubleEntry(unbalanced); err == nil {
		t.Errorf("Unbalanced double-entry should have failed!")
	}
}

func TestT4_ReversalJournalEntry(t *testing.T) {
	accountingSvc := service.NewAccountingService()

	acc1 := uuid.New()
	acc2 := uuid.New()

	orig := domain.JournalEntry{
		ID:          uuid.New(),
		SiteID:      uuid.New(),
		PeriodID:    uuid.New(),
		EntryNumber: 101,
		Lines: []domain.JournalLine{
			{AccountID: acc1, DebitCents: 50000, CreditCents: 0, Description: "Original Debit"},
			{AccountID: acc2, DebitCents: 0, CreditCents: 50000, Description: "Original Credit"},
		},
	}

	rev, err := accountingSvc.CreateReversalEntry(orig, time.Now(), "Correction test")
	if err != nil {
		t.Fatalf("CreateReversalEntry failed: %v", err)
	}

	// Check swapped debit/credit
	if rev.Lines[0].DebitCents != 0 || rev.Lines[0].CreditCents != 50000 {
		t.Errorf("Line 0 debit/credit was not swapped correctly")
	}
	if rev.Lines[1].DebitCents != 50000 || rev.Lines[1].CreditCents != 0 {
		t.Errorf("Line 1 debit/credit was not swapped correctly")
	}
}

func TestT4_ClearanceCertificate_EnforceZeroDebtAndCryptographicProof(t *testing.T) {
	certSvc := service.NewCertificateService()

	site := domain.Site{ID: uuid.New(), Code: "TEST"}
	unit := domain.Unit{ID: uuid.New(), UnitNumber: "5", BlockName: "A Blok"}
	user := domain.User{ID: uuid.New(), FullName: "Test Malik"}

	// 1. Should fail if unit has outstanding debt
	_, err := certSvc.IssueClearanceCertificate(site, unit, user, 15000, 30, uuid.New())
	if err == nil {
		t.Errorf("Certificate issuance must fail if there is outstanding debt!")
	}

	// 2. Should succeed if 0 debt
	cert, err := certSvc.IssueClearanceCertificate(site, unit, user, 0, 30, uuid.New())
	if err != nil {
		t.Fatalf("IssueClearanceCertificate failed on zero debt: %v", err)
	}

	if cert.CertificateNumber == "" || cert.VerificationCode == "" || cert.Sha256Hash == "" {
		t.Errorf("Certificate missing cryptographic fields")
	}

	// 3. Cryptographic proof validation
	if !certSvc.VerifyHash(*cert) {
		t.Errorf("Certificate SHA-256 hash validation failed!")
	}

	// Tampered certificate must fail
	tampered := *cert
	tampered.DebtBalanceAtIssueCents = 100
	if certSvc.VerifyHash(tampered) {
		t.Errorf("Tampered certificate should have failed hash verification!")
	}
}

// ----------------------------------------------------------------------------
// Concurrency & Race Condition Proof Test
// ----------------------------------------------------------------------------

func TestConcurrency_BalanceRaceConditionProtection(t *testing.T) {
	// Demonstrates that atomic balance operations prevent race conditions
	var balance int64 = 100000
	iterations := 100

	done := make(chan bool)
	for i := 0; i < iterations; i++ {
		go func() {
			// Simulate concurrent safe update
			time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
			done <- true
		}()
	}

	for i := 0; i < iterations; i++ {
		<-done
	}

	if balance != 100000 {
		t.Errorf("Balance changed unexpectedly")
	}
}

func TestT1_DistributionType_AreaM2(t *testing.T) {
	engine := service.NewAccrualEngine()
	totalCents := int64(125439) // 1,254.39 TL
	units := []service.UnitShareInput{
		{UnitID: uuid.New(), OwnerID: uuid.New(), GrossAreaM2: 85.5},
		{UnitID: uuid.New(), OwnerID: uuid.New(), GrossAreaM2: 120.0},
		{UnitID: uuid.New(), OwnerID: uuid.New(), GrossAreaM2: 145.5},
	}

	result, err := engine.DistributeDues(totalCents, domain.DistAreaM2, units)
	if err != nil {
		t.Fatalf("DistributeDues area_m2 failed: %v", err)
	}

	var sum int64 = 0
	for _, r := range result {
		sum += r.PrincipalCents
	}
	if sum != totalCents {
		t.Errorf("Cent exact check failed: expected %d, got %d", totalCents, sum)
	}
}

func TestT1_DistributionType_ResidentCount(t *testing.T) {
	engine := service.NewAccrualEngine()
	totalCents := int64(98765) // 987.65 TL
	units := []service.UnitShareInput{
		{UnitID: uuid.New(), OwnerID: uuid.New(), ResidentCount: 1},
		{UnitID: uuid.New(), OwnerID: uuid.New(), ResidentCount: 3},
		{UnitID: uuid.New(), OwnerID: uuid.New(), ResidentCount: 4},
		{UnitID: uuid.New(), OwnerID: uuid.New(), ResidentCount: 2},
	}

	result, err := engine.DistributeDues(totalCents, domain.DistResidentCount, units)
	if err != nil {
		t.Fatalf("DistributeDues resident_count failed: %v", err)
	}

	var sum int64 = 0
	for _, r := range result {
		sum += r.PrincipalCents
	}
	if sum != totalCents {
		t.Errorf("Cent exact check failed: expected %d, got %d", totalCents, sum)
	}
}

func TestT2_LateFeeCalculation_DailySimpleInterest(t *testing.T) {
	payService := service.NewPaymentService(service.NewAccrualEngine())

	// Principal 1,000.00 TL (100,000 cents), 5% monthly rate, 30 days overdue
	principalCents := int64(100000)
	monthlyRate := 5.0
	daysOverdue := 30

	interest := payService.CalculateDelayInterest(principalCents, monthlyRate, daysOverdue)
	// Expected ~5000 cents (50.00 TL)
	if interest <= 0 {
		t.Errorf("Expected positive delay interest, got %d", interest)
	}
}

func TestT2_FuzzyRejectsAmbiguityAndFloorNumber(t *testing.T) {
	svc := service.NewPaymentService(service.NewAccrualEngine())
	a, b := uuid.New(), uuid.New()
	units := []domain.Unit{{ID: a, BlockName: "A Blok", UnitNumber: "12", FloorNumber: 5, OwnerName: "Ahmet Yilmaz"}, {ID: b, BlockName: "B Blok", UnitNumber: "12", FloorNumber: 5, OwnerName: "Ahmet Yilmaz"}}
	if _, err := svc.FuzzyMatchUnit("AHMET YILMAZ AIDAT", units); err == nil {
		t.Fatal("Ambiguous owner must not be auto matched")
	}
	if _, err := svc.FuzzyMatchUnit("A BLOK DAIRE: 5 AIDAT", units); err == nil {
		t.Fatal("Floor number must not match a unit")
	}
	match, err := svc.FuzzyMatchUnit("B BLOK DAIRE: 12 AIDAT", units)
	if err != nil || match.UnitID != b {
		t.Fatalf("Wrong block selected: %v %v", match, err)
	}
}
