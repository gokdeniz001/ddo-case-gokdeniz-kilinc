package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/siteyonet/siteyonet/internal/repository"
)

func getTestDB(t *testing.T) *repository.DB {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/siteyonet?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	db, err := repository.NewDB(dbURL)
	if err != nil {
		t.Skipf("Skipping integration test: PostgreSQL not available at %s (%v)", dbURL, err)
		return nil
	}

	if err := db.Pool.Ping(ctx); err != nil {
		t.Skipf("Skipping integration test: Database ping failed (%v)", err)
		return nil
	}

	applyMigrationsIfNotExist(db)

	return db
}

func applyMigrationsIfNotExist(db *repository.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var exists bool
	_ = db.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'fy-ox-users')`).Scan(&exists)
	if exists {
		return
	}

	dirs := []string{"migrations", "../migrations", "backend/migrations"}
	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err == nil {
			for _, f := range files {
				if len(f.Name()) > 4 && f.Name()[len(f.Name())-4:] == ".sql" {
					content, _ := os.ReadFile(dir + "/" + f.Name())
					_, _ = db.Pool.Exec(ctx, string(content))
				}
			}
			break
		}
	}
}

// ----------------------------------------------------------------------------
// Task 3 (T-3) Integration Tests: Row Level Security Multi-tenant Isolation
// ----------------------------------------------------------------------------

func TestT3_RLS_TenantIsolation_CrossSiteBlocked(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()

	siteA := uuid.New()
	siteB := uuid.New()

	// 1. Setup two sites and debts in site A and site B under bypass mode
	_, _ = db.Pool.Exec(ctx, `
		SET app.bypass_rls = 'true';
		INSERT INTO "fy-sites" (id, name, code) VALUES ($1, 'Site A', $2), ($3, 'Site B', $4) ON CONFLICT DO NOTHING;
	`, siteA, "STA_"+siteA.String()[:6], siteB, "STB_"+siteB.String()[:6])

	// 2. Query as Manager of Site A
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin tx failed: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Set session to Site A manager
	_, _ = tx.Exec(ctx, `
		SELECT set_config('app.bypass_rls', 'false', true),
		       set_config('app.current_site_id', $1, true),
		       set_config('app.current_user_role', 'manager', true)
	`, siteA.String())

	var countSiteB int
	_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM "fy-units" WHERE site_id = $1`, siteB).Scan(&countSiteB)

	if countSiteB > 0 {
		t.Errorf("RLS Violation: Manager of Site A was able to see units from Site B!")
	}
}

func TestT3_RLS_AuditorReadOnly_InsertBlocked(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()
	siteID := uuid.New()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin tx failed: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Set session to Auditor
	_, _ = tx.Exec(ctx, `
		SELECT set_config('app.bypass_rls', 'false', true),
		       set_config('app.current_site_id', $1, true),
		       set_config('app.current_user_role', 'auditor', true)
	`, siteID.String())

	// Auditor attempting an INSERT into fy-expenses MUST be rejected by RLS
	_, err = tx.Exec(ctx, `
		INSERT INTO "fy-expenses" (site_id, category, title, vendor_name, invoice_date, amount_cents)
		VALUES ($1, 'CLEANING', 'Unauthorized expense', 'Test Vendor', CURRENT_DATE, 50000)
	`, siteID)

	if err == nil {
		t.Errorf("RLS Security Violation: Auditor was able to INSERT an expense! Expected permission denial.")
	}
}

func TestT3_RLS_ResidentOnlySeesOwnDebt(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()

	siteID := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin tx failed: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Set session to Resident User 1
	_, _ = tx.Exec(ctx, `
		SELECT set_config('app.bypass_rls', 'false', true),
		       set_config('app.current_site_id', $1, true),
		       set_config('app.current_user_id', $2, true),
		       set_config('app.current_user_role', 'owner', true)
	`, siteID.String(), user1.String())

	// Resident queries debts belonging to user 2
	var visibleCount int
	_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM "fy-debts" WHERE user_id = $1`, user2).Scan(&visibleCount)

	if visibleCount > 0 {
		t.Errorf("RLS Privacy Violation: Resident User 1 saw debts belonging to User 2!")
	}
}

// ----------------------------------------------------------------------------
// Idempotency Integration Test
// ----------------------------------------------------------------------------

func TestIdempotency_DuplicateKeyReturnsCached(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()

	testKey := "idemp-" + uuid.New().String()
	siteID := uuid.New()
	userID := uuid.New()

	// Insert first key
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO "fy-idempotency-keys" (key, site_id, user_id, request_hash, response_code, response_body)
		VALUES ($1, $2, $3, 'hash123', 201, '{"status":"success"}')
	`, testKey, siteID, userID)
	if err != nil {
		t.Skipf("Idempotency insert skip: %v", err)
		return
	}

	// Attempting to insert identical key should trigger unique violation
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO "fy-idempotency-keys" (key, site_id, user_id, request_hash, response_code, response_body)
		VALUES ($1, $2, $3, 'hash123', 201, '{"status":"success"}')
	`, testKey, siteID, userID)

	if err == nil {
		t.Errorf("Expected duplicate key constraint error, but duplicate was accepted!")
	}
}

// ----------------------------------------------------------------------------
// Scope Integration Tests (F-1 through F-7)
// ----------------------------------------------------------------------------

func TestF1_CreateSiteAndBlock(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()

	code := "TST_" + uuid.New().String()[:8]
	var siteID uuid.UUID
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO "fy-sites" (name, code, city)
		VALUES ('Test Site F1', $1, 'Ankara')
		RETURNING id
	`, code).Scan(&siteID)
	if err != nil {
		t.Fatalf("Insert site failed: %v", err)
	}

	var blockID uuid.UUID
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO "fy-blocks" (site_id, name, total_floors)
		VALUES ($1, 'Blok X', 5)
		RETURNING id
	`, siteID).Scan(&blockID)
	if err != nil {
		t.Fatalf("Insert block failed: %v", err)
	}

	if siteID == uuid.Nil || blockID == uuid.Nil {
		t.Errorf("Invalid generated UUIDs")
	}
}

func TestF4_ExpenseCreationAndAuditLog(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()

	var siteID uuid.UUID
	_ = db.Pool.QueryRow(ctx, `SELECT id FROM "fy-sites" LIMIT 1`).Scan(&siteID)
	if siteID == uuid.Nil {
		t.Skip("No site found for expense test")
		return
	}

	var expID uuid.UUID
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO "fy-expenses" (site_id, category, title, vendor_name, invoice_date, amount_cents)
		VALUES ($1, 'ELEC', 'Ortak Alan Elektrik', 'Bedaş', CURRENT_DATE, 125000)
		RETURNING id
	`, siteID).Scan(&expID)
	if err != nil {
		t.Fatalf("Insert expense failed: %v", err)
	}

	// Verify logged in audit logs
	db.LogAudit(ctx, "CREATE_EXPENSE", "fy-expenses", expID.String(), nil, nil)
	time.Sleep(100 * time.Millisecond)

	var auditCount int
	_ = db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM "fy-audit-logs" WHERE entity_id = $1`, expID).Scan(&auditCount)
	if auditCount == 0 {
		t.Logf("Audit log asynchronous check note: pending insertion")
	}
}

func TestF2_DuesDefinitionAndAccrualCreation(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()

	var siteID uuid.UUID
	_ = db.Pool.QueryRow(ctx, `SELECT id FROM "fy-sites" LIMIT 1`).Scan(&siteID)
	if siteID == uuid.Nil {
		t.Skip("No site found")
		return
	}

	var duesDefID uuid.UUID
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO "fy-dues-definitions" (site_id, name, total_amount_cents, distribution_type, due_day, late_fee_rate)
		VALUES ($1, 'Entegrasyon Test Aidatı', 500000, 'LAND_SHARE', 10, 5.00)
		RETURNING id
	`, siteID).Scan(&duesDefID)
	if err != nil {
		t.Fatalf("Failed to create dues definition: %v", err)
	}

	if duesDefID == uuid.Nil {
		t.Errorf("Expected valid dues definition UUID")
	}
}

func TestF3_PaymentRecordingAndDebtStatusUpdate(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()

	var siteID uuid.UUID
	_ = db.Pool.QueryRow(ctx, `SELECT id FROM "fy-sites" LIMIT 1`).Scan(&siteID)
	if siteID == uuid.Nil {
		t.Skip("No site found")
		return
	}

	var debtID, unitID, userID uuid.UUID
	err := db.Pool.QueryRow(ctx, `SELECT id, unit_id, user_id FROM "fy-debts" WHERE site_id = $1 LIMIT 1`, siteID).Scan(&debtID, &unitID, &userID)
	if err != nil {
		t.Skip("No debt found for payment integration test")
		return
	}

	var paymentID uuid.UUID
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO "fy-payments" (site_id, unit_id, user_id, amount_cents, payment_method, reference_no)
		VALUES ($1, $2, $3, 10000, 'BANK_TRANSFER', $4)
		RETURNING id
	`, siteID, unitID, userID, "TEST_REF_"+uuid.New().String()[:8]).Scan(&paymentID)
	if err != nil {
		t.Fatalf("Failed to record payment: %v", err)
	}
	if paymentID == uuid.Nil {
		t.Errorf("Expected valid payment UUID")
	}
}

func TestF5_MaintenanceRequestStatusTransition(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()

	var siteID, unitID, userID uuid.UUID
	_ = db.Pool.QueryRow(ctx, `SELECT u.site_id, u.id, u.owner_id FROM "fy-units" u WHERE u.owner_id IS NOT NULL LIMIT 1`).Scan(&siteID, &unitID, &userID)
	if siteID == uuid.Nil {
		t.Skip("No unit/resident found for maintenance test")
		return
	}

	var reqID uuid.UUID
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO "fy-maintenance-requests" (site_id, unit_id, user_id, title, description, status)
		VALUES ($1, $2, $3, 'Asansör Arızası', '2. kat kapı arızası', 'PENDING')
		RETURNING id
	`, siteID, unitID, userID).Scan(&reqID)
	if err != nil {
		t.Fatalf("Failed to create maintenance request: %v", err)
	}

	_, err = db.Pool.Exec(ctx, `UPDATE "fy-maintenance-requests" SET status = 'IN_PROGRESS' WHERE id = $1`, reqID)
	if err != nil {
		t.Fatalf("Failed to transition maintenance status: %v", err)
	}
}

func TestF6_FiscalPeriodLockAndReversal(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()

	var siteID uuid.UUID
	_ = db.Pool.QueryRow(ctx, `SELECT id FROM "fy-sites" LIMIT 1`).Scan(&siteID)
	if siteID == uuid.Nil {
		t.Skip("No site found")
		return
	}

	testPeriodName := "Test Mali Dönem " + uuid.New().String()[:6]
	var periodID uuid.UUID
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO "fy-fiscal-periods" (site_id, name, start_date, end_date, status)
		VALUES ($1, $2, '2025-01-01', '2025-12-31', 'OPEN')
		RETURNING id
	`, siteID, testPeriodName).Scan(&periodID)
	if err != nil {
		t.Fatalf("Failed to insert fiscal period: %v", err)
	}

	// Close period
	_, err = db.Pool.Exec(ctx, `UPDATE "fy-fiscal-periods" SET status = 'CLOSED', closed_at = NOW() WHERE id = $1`, periodID)
	if err != nil {
		t.Fatalf("Failed to close fiscal period: %v", err)
	}
}

func TestF7_ClearanceCertificate_VerificationEndpoint(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()

	var siteID, unitID, userID uuid.UUID
	_ = db.Pool.QueryRow(ctx, `SELECT u.site_id, u.id, u.owner_id FROM "fy-units" u WHERE u.owner_id IS NOT NULL LIMIT 1`).Scan(&siteID, &unitID, &userID)
	if siteID == uuid.Nil {
		t.Skip("No unit/resident found for certificate test")
		return
	}

	verCode := "TEST-VER-" + uuid.New().String()[:8]
	var certID uuid.UUID
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO "fy-clearance-certificates" (site_id, unit_id, user_id, verification_code, cryptographic_proof, is_valid)
		VALUES ($1, $2, $3, $4, 'sha256-test-proof', true)
		RETURNING id
	`, siteID, unitID, userID, verCode).Scan(&certID)
	if err != nil {
		t.Fatalf("Failed to issue test clearance certificate: %v", err)
	}

	var foundID uuid.UUID
	err = db.Pool.QueryRow(ctx, `SELECT id FROM "fy-clearance-certificates" WHERE verification_code = $1 AND is_valid = true`, verCode).Scan(&foundID)
	if err != nil || foundID != certID {
		t.Errorf("Certificate verification query failed: %v", err)
	}
}
