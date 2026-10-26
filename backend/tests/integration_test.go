package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/siteyonet/siteyonet/internal/handler"
	"github.com/siteyonet/siteyonet/internal/middleware"
	"github.com/siteyonet/siteyonet/internal/repository"
	"github.com/siteyonet/siteyonet/internal/service"
	"golang.org/x/crypto/bcrypt"
)

type fixture struct {
	db                                                             *repository.DB
	api                                                            http.Handler
	site, manager, owner, other, unit, period, debit, credit, debt uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Fatal("DATABASE_URL required: integration tests must use PostgreSQL")
	}
	db, err := repository.NewDB(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Pool.Close)
	if err := db.Migrate("../migrations"); err != nil {
		t.Fatal(err)
	}
	f := &fixture{db: db, site: uuid.New(), manager: uuid.New(), owner: uuid.New(), other: uuid.New(), unit: uuid.New(), period: uuid.New(), debit: uuid.New(), credit: uuid.New(), debt: uuid.New()}
	ctx := context.Background()
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := db.Pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO "fy-sites" (id,name,code) VALUES ($1,'Integration site',$2)`, f.site, "IT-"+f.site.String())
	t.Cleanup(func() {
		_, _ = db.Pool.Exec(ctx, `DELETE FROM "fy-journal-entries" WHERE site_id=$1`, f.site)
		_, err := db.Pool.Exec(ctx, `DELETE FROM "fy-sites" WHERE id=$1`, f.site)
		_, _ = db.Pool.Exec(ctx, `DELETE FROM "fy-ox-users" WHERE id IN ($1,$2,$3)`, f.manager, f.owner, f.other)
		if err != nil {
			t.Error(err)
		}
	})
	for _, u := range []struct {
		id   uuid.UUID
		role string
	}{{f.manager, "manager"}, {f.owner, "owner"}, {f.other, "owner"}} {
		exec(`INSERT INTO "fy-ox-users" (id,site_id,email,password_hash,full_name,role) VALUES ($1,$2,$3,'unused','Test user',$4)`, u.id, f.site, u.id.String()+"@test.invalid", u.role)
	}
	block := uuid.New()
	exec(`INSERT INTO "fy-blocks" (id,site_id,name) VALUES ($1,$2,'A Blok')`, block, f.site)
	exec(`INSERT INTO "fy-units" (id,site_id,block_id,unit_number,owner_id,land_share,gross_area_m2,resident_count) VALUES ($1,$2,$3,'5',$4,100,100,2)`, f.unit, f.site, block, f.owner)
	exec(`INSERT INTO "fy-fiscal-periods" (id,site_id,name,start_date,end_date) VALUES ($1,$2,'Test','2026-01-01','2026-12-31')`, f.period, f.site)
	exec(`INSERT INTO "fy-ledger-accounts" (id,site_id,code,name,account_type) VALUES ($1,$2,'102','Bank','ASSET'),($3,$2,'120','Receivable','ASSET')`, f.debit, f.site, f.credit)
	exec(`INSERT INTO "fy-debts" (id,site_id,unit_id,user_id,title,debt_type,term_month,principal_cents,due_date) VALUES ($1,$2,$3,$4,'Test','DUES','2026-10',10000,'2026-10-26')`, f.debt, f.site, f.unit, f.owner)
	exec(`INSERT INTO "fy-ledger-accounts" (site_id,code,name,account_type) VALUES ($1,'340','Advance','LIABILITY'),($1,'642','Interest','REVENUE'),($1,'100','Cash','ASSET'),($1,'600','Dues','REVENUE'),($1,'770','Expense','EXPENSE'),($1,'320','Payable','LIABILITY')`, f.site)
	engine := service.NewAccrualEngine()
	f.api = handler.NewHandler(db, engine, service.NewPaymentService(engine), service.NewAccountingService(), service.NewCertificateService()).InitRoutes()
	return f
}
func (f *fixture) request(t *testing.T, method, path, role string, user uuid.UUID, body interface{}, key string) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	if role != "" {
		tok, err := middleware.GenerateToken(user, f.site, role, "test@test.invalid", "Test", time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	f.api.ServeHTTP(w, r)
	return w
}
func expectStatus(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("HTTP %d, want %d: %s", w.Code, status, w.Body.String())
	}
}
func (f *fixture) payment(t *testing.T, key string, amount int) *httptest.ResponseRecorder {
	return f.request(t, "POST", "/api/v1/payments", "manager", f.manager, map[string]interface{}{"unit_id": f.unit, "amount_cents": amount, "payment_date": "2026-10-26", "payment_channel": "BANK_TRANSFER"}, key)
}
func TestIntegration_AnonymousEndpointsDenied(t *testing.T) {
	f := newFixture(t)
	for _, p := range []string{"sites", "units", "debts", "expenses", "periods", "journal-entries", "certificates", "announcements", "maintenance"} {
		expectStatus(t, f.request(t, "GET", "/api/v1/"+p, "", uuid.Nil, nil, ""), 401)
	}
}
func TestIntegration_ResidentUnitPrivacy(t *testing.T) {
	f := newFixture(t)
	w := f.request(t, "GET", "/api/v1/units", "owner", f.other, nil, "")
	expectStatus(t, w, 200)
	var rows []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("Unrelated owner saw units: %s", w.Body.String())
	}
}
func TestIntegration_ResidentDebtPrivacy(t *testing.T) {
	f := newFixture(t)
	w := f.request(t, "GET", "/api/v1/debts", "owner", f.other, nil, "")
	expectStatus(t, w, 200)
	var rows []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatal("Resident saw another user's debt")
	}
	w = f.request(t, "GET", "/api/v1/debts", "owner", f.owner, nil, "")
	expectStatus(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("Owner must see their own debt: %s", w.Body.String())
	}
}
func TestIntegration_RLSCrossSiteBlocked(t *testing.T) {
	f := newFixture(t)
	other := newFixture(t)
	ctx := context.WithValue(context.Background(), middleware.UserContextKey, &middleware.UserClaims{UserID: f.manager, SiteID: f.site, Role: "manager"})
	ctx, tx, err := f.db.BeginTenant(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM "fy-units" WHERE site_id=$1`, other.site).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("Cross-site RLS leak")
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM "fy-units" WHERE site_id=$1`, f.site).Scan(&n); err != nil || n != 1 {
		t.Fatalf("Own site must remain visible: %d %v", n, err)
	}
}
func TestIntegration_AuditorCannotWrite(t *testing.T) {
	f := newFixture(t)
	expectStatus(t, f.request(t, "POST", "/api/v1/expenses", "auditor", f.manager, map[string]interface{}{"title": "Denied", "amount_cents": 100}, ""), 403)
	ctx := context.WithValue(context.Background(), middleware.UserContextKey, &middleware.UserClaims{UserID: f.manager, SiteID: f.site, Role: "auditor"})
	ctx, tx, err := f.db.BeginTenant(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO "fy-expenses" (site_id,category,title,vendor_name,invoice_date,amount_cents) VALUES ($1,'CLEANING','Denied','Test',CURRENT_DATE,100)`, f.site)
	if err == nil {
		t.Fatal("Database accepted auditor write")
	}
}
func TestIntegration_PaymentAndAudit(t *testing.T) {
	f := newFixture(t)
	expectStatus(t, f.payment(t, "", 1000), 201)
	var paid, n int
	if err := f.db.Pool.QueryRow(context.Background(), `SELECT paid_principal_cents FROM "fy-debts" WHERE id=$1`, f.debt).Scan(&paid); err != nil {
		t.Fatal(err)
	}
	if paid != 1000 {
		t.Fatalf("Paid %d want 1000", paid)
	}
	if err := f.db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "fy-audit-logs" WHERE site_id=$1 AND action='PROCESS_PAYMENT'`, f.site).Scan(&n); err != nil || n != 1 {
		t.Fatalf("Audit missing: %d %v", n, err)
	}
}
func TestIntegration_IdempotencyReplayAndConflict(t *testing.T) {
	f := newFixture(t)
	a := f.payment(t, "same", 1000)
	expectStatus(t, a, 201)
	b := f.payment(t, "same", 1000)
	expectStatus(t, b, 201)
	var x, y map[string]interface{}
	if err := json.Unmarshal(a.Body.Bytes(), &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b.Body.Bytes(), &y); err != nil {
		t.Fatal(err)
	}
	if x["id"] != y["id"] {
		t.Fatal("Replay created another payment")
	}
	expectStatus(t, f.payment(t, "same", 2000), 409)
}
func TestIntegration_ConcurrentPayments(t *testing.T) {
	f := newFixture(t)
	var wg sync.WaitGroup
	codes := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); codes <- f.payment(t, fmt.Sprintf("pay-%d", i), 2000).Code }(i)
	}
	wg.Wait()
	close(codes)
	for c := range codes {
		if c != 201 {
			t.Fatalf("Concurrent payment returned %d", c)
		}
	}
	var paid, credit int64
	if err := f.db.Pool.QueryRow(context.Background(), `SELECT paid_principal_cents FROM "fy-debts" WHERE id=$1`, f.debt).Scan(&paid); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Pool.QueryRow(context.Background(), `SELECT sum(unallocated_cents) FROM "fy-payments" WHERE site_id=$1`, f.site).Scan(&credit); err != nil {
		t.Fatal(err)
	}
	if paid != 10000 || credit != 6000 {
		t.Fatalf("Lost/duplicated balance: paid=%d credit=%d", paid, credit)
	}
}
func TestIntegration_MaintenanceTransition(t *testing.T) {
	f := newFixture(t)
	w := f.request(t, "POST", "/api/v1/maintenance", "owner", f.owner, map[string]interface{}{"unit_id": f.unit, "title": "Elevator", "description": "Door", "category": "OTHER", "priority": "MEDIUM"}, "")
	expectStatus(t, w, 201)
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, f.request(t, "PUT", "/api/v1/maintenance/"+result.ID+"/status", "manager", f.manager, map[string]interface{}{"status": "IN_PROGRESS"}, ""), 200)
}
func TestIntegration_ClosedPeriodRejectsJournal(t *testing.T) {
	f := newFixture(t)
	expectStatus(t, f.request(t, "POST", "/api/v1/periods/"+f.period.String()+"/close", "manager", f.manager, nil, ""), 200)
	w := f.request(t, "POST", "/api/v1/journal-entries", "manager", f.manager, map[string]interface{}{"period_id": f.period, "entry_date": "2026-10-26", "lines": []map[string]interface{}{{"AccountID": f.debit, "DebitCents": 100}, {"AccountID": f.credit, "CreditCents": 100}}}, "")
	expectStatus(t, w, 400)
	if !bytes.Contains(w.Body.Bytes(), []byte("LOCKED_PERIOD")) {
		t.Fatalf("Wrong rejection: %s", w.Body.String())
	}
}
func TestIntegration_CertificateDebtAndVerification(t *testing.T) {
	f := newFixture(t)
	body := map[string]interface{}{"unit_id": f.unit, "valid_days": 30}
	expectStatus(t, f.request(t, "POST", "/api/v1/certificates", "manager", f.manager, body, ""), 400)
	expectStatus(t, f.payment(t, "clear", 10000), 201)
	w := f.request(t, "POST", "/api/v1/certificates", "manager", f.manager, body, "")
	expectStatus(t, w, 201)
	var cert struct {
		Code string `json:"verification_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cert); err != nil {
		t.Fatal(err)
	}
	w = f.request(t, "GET", "/api/v1/certificates/verify/"+cert.Code, "", uuid.Nil, nil, "")
	expectStatus(t, w, 200)
	var result struct {
		Valid bool `json:"validity_status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || !result.Valid {
		t.Fatalf("Invalid certificate: %s (%v)", w.Body.String(), err)
	}
}

func TestIntegration_AccrualCentExactAPI(t *testing.T) {
	f := newFixture(t)
	w := f.request(t, "POST", "/api/v1/accruals", "manager", f.manager, map[string]interface{}{"period_id": f.period, "title": "Exact", "term_month": "2026-10", "distribution_type": "LAND_SHARE", "total_amount_cents": 125433, "due_day": 15}, "")
	expectStatus(t, w, 201)
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var sum int64
	if err := f.db.Pool.QueryRow(context.Background(), `SELECT sum(principal_cents) FROM "fy-debts" WHERE accrual_id=$1`, result.ID).Scan(&sum); err != nil || sum != 125433 {
		t.Fatalf("Allocation mismatch %d: %v", sum, err)
	}
}
func TestIntegration_RestructureAPI(t *testing.T) {
	f := newFixture(t)
	expectStatus(t, f.request(t, "POST", "/api/v1/debts/restructure", "manager", f.manager, map[string]interface{}{"unit_id": f.unit, "installment_count": 3}, ""), 201)
	var total int64
	var count int
	if err := f.db.Pool.QueryRow(context.Background(), `SELECT count(*),sum(principal_cents) FROM "fy-debts" WHERE site_id=$1 AND debt_type='RESTRUCTURE'`, f.site).Scan(&count, &total); err != nil || count != 3 || total != 10000 {
		t.Fatalf("Invalid plan count=%d total=%d err=%v", count, total, err)
	}
}
func TestIntegration_IdempotencyConcurrentSameKey(t *testing.T) {
	f := newFixture(t)
	var wg sync.WaitGroup
	codes := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- f.payment(t, "once", 1000).Code }()
	}
	wg.Wait()
	close(codes)
	for c := range codes {
		if c != 201 {
			t.Fatalf("Concurrent replay returned %d", c)
		}
	}
	var n int
	if err := f.db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "fy-payments" WHERE site_id=$1`, f.site).Scan(&n); err != nil || n != 1 {
		t.Fatalf("Duplicate business effect n=%d err=%v", n, err)
	}
}
func TestIntegration_CrossSiteUnitReferenceRejected(t *testing.T) {
	f := newFixture(t)
	other := newFixture(t)
	w := f.request(t, "POST", "/api/v1/payments", "manager", f.manager, map[string]interface{}{"unit_id": other.unit, "amount_cents": 1000, "payment_date": "2026-10-26", "payment_channel": "BANK_TRANSFER"}, "")
	if w.Code < 400 {
		t.Fatalf("Foreign unit accepted: %s", w.Body.String())
	}
	var n int
	if err := f.db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "fy-payments" WHERE site_id=$1`, f.site).Scan(&n); err != nil || n != 0 {
		t.Fatalf("Foreign payment persisted: %d %v", n, err)
	}
}

func TestIntegration_RefreshTokenRotation(t *testing.T) {
	f := newFixture(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Pool.Exec(context.Background(), `UPDATE "fy-ox-users" SET password_hash=$1 WHERE id=$2`, string(hash), f.manager); err != nil {
		t.Fatal(err)
	}
	w := f.request(t, "POST", "/api/v1/auth/login", "", uuid.Nil, map[string]interface{}{"email": f.manager.String() + "@test.invalid", "password": "test-password"}, "")
	expectStatus(t, w, 200)
	var session struct {
		Token, Refresh string `json:"-"`
	}
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	session.Refresh, _ = result["refresh_token"].(string)
	if session.Refresh == "" {
		t.Fatal("Missing refresh token")
	}
	w = f.request(t, "POST", "/api/v1/auth/refresh", "", uuid.Nil, map[string]interface{}{"refresh_token": session.Refresh}, "")
	expectStatus(t, w, 200)
	expectStatus(t, f.request(t, "POST", "/api/v1/auth/refresh", "", uuid.Nil, map[string]interface{}{"refresh_token": session.Refresh}, ""), 401)
}
func TestIntegration_MultiSiteSwitch(t *testing.T) {
	f := newFixture(t)
	other := newFixture(t)
	expectStatus(t, f.request(t, "POST", "/api/v1/auth/site", "manager", f.manager, map[string]interface{}{"site_id": other.site}, ""), 403)
	if _, err := f.db.Pool.Exec(context.Background(), `INSERT INTO "fy-user-sites" (user_id,site_id,role) VALUES ($1,$2,'manager')`, f.manager, other.site); err != nil {
		t.Fatal(err)
	}
	w := f.request(t, "POST", "/api/v1/auth/site", "manager", f.manager, map[string]interface{}{"site_id": other.site}, "")
	expectStatus(t, w, 200)
	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/v1/units", nil)
	r.Header.Set("Authorization", "Bearer "+result.Token)
	w = httptest.NewRecorder()
	f.api.ServeHTTP(w, r)
	expectStatus(t, w, 200)
	var rows []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != other.unit.String() {
		t.Fatalf("Wrong tenant after switch: %s", w.Body.String())
	}
}

func TestIntegration_OwnershipDaySplitAPI(t *testing.T) {
	f := newFixture(t)
	expectStatus(t, f.request(t, "POST", "/api/v1/units/"+f.unit.String()+"/transfer", "manager", f.manager, map[string]interface{}{"new_owner_id": f.other, "transfer_date": "2026-10-11"}, ""), 200)
	w := f.request(t, "POST", "/api/v1/accruals", "manager", f.manager, map[string]interface{}{"period_id": f.period, "title": "Transfer", "term_month": "2026-10", "distribution_type": "EQUAL", "total_amount_cents": 3100, "due_day": 15}, "")
	expectStatus(t, w, 201)
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var old, newAmount int64
	if err := f.db.Pool.QueryRow(context.Background(), `SELECT sum(principal_cents) FILTER (WHERE user_id=$2),sum(principal_cents) FILTER (WHERE user_id=$3) FROM "fy-debts" WHERE accrual_id=$1`, result.ID, f.owner, f.other).Scan(&old, &newAmount); err != nil || old != 1000 || newAmount != 2100 {
		t.Fatalf("Wrong day allocation old=%d new=%d err=%v", old, newAmount, err)
	}
}

func TestIntegration_DebtKeysetPagination(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.db.Pool.Exec(ctx, `INSERT INTO "fy-debts" (site_id,unit_id,user_id,title,debt_type,term_month,principal_cents,due_date) VALUES ($1,$2,$3,'Next','DUES','2026-10',100,'2026-10-26')`, f.site, f.unit, f.owner); err != nil {
		t.Fatal(err)
	}
	var first, second []struct {
		ID  string `json:"id"`
		Due string `json:"due_date"`
	}
	w := f.request(t, "GET", "/api/v1/debts?limit=1", "manager", f.manager, nil, "")
	expectStatus(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &first); err != nil || len(first) != 1 {
		t.Fatalf("Invalid first page %s", w.Body.String())
	}
	w = f.request(t, "GET", "/api/v1/debts?limit=1&after="+first[0].Due[:10]+","+first[0].ID, "manager", f.manager, nil, "")
	expectStatus(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &second); err != nil || len(second) != 1 || second[0].ID == first[0].ID {
		t.Fatalf("Invalid second page %s", w.Body.String())
	}
}

func TestIntegration_BankCSVImport(t *testing.T) {
	f := newFixture(t)
	w := f.request(t, "POST", "/api/v1/bank-statements/import", "manager", f.manager, map[string]interface{}{"csv": "date,reference,description,amount_cents\n2026-10-26,REF-1,A BLOK DAIRE: 5 AIDAT,10000\n2026-10-26,REF-2,BELIRSIZ DEKONT,2000\n"}, "csv-once")
	expectStatus(t, w, 201)
	var result struct {
		Imported int `json:"imported"`
		Matched  int `json:"matched"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Imported != 2 || result.Matched < 1 {
		t.Fatalf("CSV import failed: %s %v", w.Body.String(), err)
	}
	var n int
	if err := f.db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "fy-bank-statements" WHERE site_id=$1`, f.site).Scan(&n); err != nil || n != 2 {
		t.Fatalf("Unexpected row count %d %v", n, err)
	}
	if _, err := f.db.Pool.Exec(context.Background(), `INSERT INTO "fy-bank-statements" (site_id,statement_date,raw_description,amount_cents) VALUES ($1,'2026-10-26','Legacy unmatched',1000)`, f.site); err != nil {
		t.Fatal(err)
	}
	w = f.request(t, "GET", "/api/v1/bank-statements", "manager", f.manager, nil, "")
	expectStatus(t, w, 200)
	var listed []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil || len(listed) != 3 {
		t.Fatalf("NULL confidence hid statement rows: %s %v", w.Body.String(), err)
	}

}

func TestIntegration_BudgetAndReconciliation(t *testing.T) {
	f := newFixture(t)
	expectStatus(t, f.request(t, "POST", "/api/v1/budgets", "manager", f.manager, map[string]interface{}{"category": "CLEANING", "amount_cents": 10000}, ""), 200)
	expectStatus(t, f.request(t, "POST", "/api/v1/expenses", "manager", f.manager, map[string]interface{}{"period_id": f.period, "category": "CLEANING", "title": "Test", "vendor_name": "Vendor", "invoice_date": "2026-10-26", "amount_cents": 1000}, ""), 201)
	w := f.request(t, "GET", "/api/v1/reports/budget-comparison", "manager", f.manager, nil, "")
	expectStatus(t, w, 200)
	var rows []struct {
		Variance int64 `json:"variance_cents"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0].Variance != 9000 {
		t.Fatalf("Wrong budget: %s", w.Body.String())
	}
	w = f.request(t, "GET", "/api/v1/reports/reconciliation", "manager", f.manager, nil, "")
	expectStatus(t, w, 200)
	var result struct {
		Bank int64 `json:"book_bank_cents"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Bank != 0 {
		t.Fatalf("Unpaid expense changed bank: %s", w.Body.String())
	}
}
func TestIntegration_AccrualReversalLedger(t *testing.T) {
	f := newFixture(t)
	w := f.request(t, "POST", "/api/v1/accruals", "manager", f.manager, map[string]interface{}{"period_id": f.period, "title": "Reverse", "term_month": "2026-10", "distribution_type": "EQUAL", "total_amount_cents": 3100, "due_day": 15}, "")
	expectStatus(t, w, 201)
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, f.request(t, "POST", "/api/v1/accruals/"+result.ID+"/reverse", "manager", f.manager, nil, ""), 200)
	var balance int64
	if err := f.db.Pool.QueryRow(context.Background(), `SELECT COALESCE(sum(l.debit_cents-l.credit_cents),0) FROM "fy-journal-lines" l JOIN "fy-ledger-accounts" a ON a.id=l.account_id WHERE a.site_id=$1 AND a.code='120'`, f.site).Scan(&balance); err != nil || balance != 0 {
		t.Fatalf("Accrual reversal left ledger balance %d %v", balance, err)
	}
	expectStatus(t, f.request(t, "POST", "/api/v1/accruals/"+result.ID+"/reverse", "manager", f.manager, nil, ""), 409)
}
