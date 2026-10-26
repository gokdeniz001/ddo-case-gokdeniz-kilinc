package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/siteyonet/siteyonet/internal/config"
	"github.com/siteyonet/siteyonet/internal/domain"
	"github.com/siteyonet/siteyonet/internal/middleware"
	"github.com/siteyonet/siteyonet/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	log.Println("Starting SiteYonet Data Simulator & Seeder...")

	cfg := config.Load()
	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Simulator: Failed to connect to DB: %v", err)
	}
	defer db.Pool.Close()

	// Seed source for deterministic reproducibility
	seedVal := int64(42)
	if sEnv := os.Getenv("SIMULATOR_SEED"); sEnv != "" {
		if val, err := strconv.ParseInt(sEnv, 10, 64); err == nil {
			seedVal = val
		}
	}
	r := rand.New(rand.NewSource(seedVal))
	log.Printf("Using deterministic random seed: %d", seedVal)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	ctx = context.WithValue(ctx, middleware.TransactionKey, tx)
	// Bypass RLS during initial seed
	_, _ = db.Executor(ctx).Exec(ctx, "SELECT set_config('app.bypass_rls', 'true', false)")

	// 1. Check if seed already done
	var existingUsersCount int
	_ = db.Executor(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM "fy-ox-users"`).Scan(&existingUsersCount)
	if existingUsersCount > 0 {
		if os.Getenv("FORCE_SEED") != "true" {
			log.Printf("Database already contains %d users. Skipping re-seed. (Set FORCE_SEED=true to override)", existingUsersCount)
			return
		}
		log.Println("FORCE_SEED=true: Truncating existing data for clean re-seed...")
		_, _ = db.Executor(ctx).Exec(ctx, `
			TRUNCATE TABLE "fy-payment-allocations", "fy-payments", "fy-debts", "fy-accruals", "fy-dues-definitions",
			               "fy-expenses", "fy-bank-statements", "fy-clearance-certificates", "fy-announcements",
			               "fy-decisions", "fy-maintenance-requests", "fy-journal-lines", "fy-journal-entries",
			               "fy-ledger-accounts", "fy-units", "fy-blocks", "fy-user-sites", "fy-ox-users", "fy-fiscal-periods", "fy-sites" CASCADE
		`)
	}

	log.Println("Generating deterministic seed data (Sites, Units, Users for all roles, Accruals, Payments)...")

	// Standard hashed password "password123"
	hashedPass, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	passStr := string(hashedPass)

	// Determine how many sites: full mode produces 40 sites, default demo produces 5 high-fidelity sites
	siteCount := 40
	if os.Getenv("FULL_SIMULATION") == "false" {
		siteCount = 5
	}

	siteNames := []string{
		"Güneş Park Evleri", "Mavi Vadi Sitesi", "Boğaziçi Konakları", "Ihlamur Rezidans", "Çamlıca Villaları",
		"Zümrüt Yaşam Evleri", "Yıldız Park Sitesi", "Defne Konutları", "Akasya Evleri", "Nilüfer Rezidans",
	}

	for i := 0; i < siteCount; i++ {
		siteName := fmt.Sprintf("Site-%02d", i+1)
		if i < len(siteNames) {
			siteName = siteNames[i]
		}
		siteCode := fmt.Sprintf("STE%02d", i+1)
		iban := fmt.Sprintf("TR%02d000620000000000000%04d", 10+i, 1000+i)

		var siteID uuid.UUID
		err := db.Executor(ctx).QueryRow(ctx, `
			INSERT INTO "fy-sites" (name, code, address, city, district, bank_name, bank_iban, default_late_fee_rate)
			VALUES ($1, $2, $3, 'İstanbul', 'Kadıköy', 'Garanti BBVA', $4, 5.00)
			ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		`, siteName, siteCode, siteName+" Mah. 123. Sok No:"+strconv.Itoa(i+1), iban).Scan(&siteID)
		if err != nil {
			log.Fatalf("Error inserting site %s: %v", siteName, err)
		}

		// Create standard chart of accounts for this site
		seedAccounts(ctx, db, siteID)

		// Create open fiscal period
		var periodID uuid.UUID
		_ = db.Executor(ctx).QueryRow(ctx, `
			INSERT INTO "fy-fiscal-periods" (site_id, name, start_date, end_date, status)
			VALUES ($1, '2026 Mali Yılı', '2026-01-01', '2026-12-31', 'OPEN')
			ON CONFLICT (site_id, start_date, end_date) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		`, siteID).Scan(&periodID)

		// Create standard role demo accounts on primary site (Site 1)
		var managerID, ownerID uuid.UUID
		prefix := fmt.Sprintf("site%d", i+1)

		if i == 0 {
			// DDO Case Standardı 5: Seed verisinde ilk kullanıcının e-postası seed.kumsal@ddo-case.invalid olsun
			_ = seedUser(ctx, db, siteID, "seed.kumsal@ddo-case.invalid", passStr, "Kumsal Seed Kullanıcısı", domain.RoleSuperAdmin)
		}

		managerID = seedUser(ctx, db, siteID, prefix+"_manager@siteyonet.com", passStr, siteName+" Yöneticisi", domain.RoleManager)
		_ = seedUser(ctx, db, siteID, prefix+"_accountant@siteyonet.com", passStr, siteName+" Muhasebecisi", domain.RoleAccountant)
		_ = seedUser(ctx, db, siteID, prefix+"_auditor@siteyonet.com", passStr, siteName+" Denetçisi", domain.RoleAuditor)
		ownerID = seedUser(ctx, db, siteID, prefix+"_owner@siteyonet.com", passStr, "Ahmet Yılmaz (Malik)", domain.RoleOwner)
		_ = seedUser(ctx, db, siteID, prefix+"_tenant@siteyonet.com", passStr, "Mehmet Demir (Kiracı)", domain.RoleTenant)

		// Also seed canonical root accounts for easy evaluation:
		if i == 0 {
			_ = seedUser(ctx, db, siteID, "admin@siteyonet.com", passStr, "Sistem Süper Yöneticisi", domain.RoleSuperAdmin)
			_ = seedUser(ctx, db, siteID, "yonetici@siteyonet.com", passStr, "Ali Yönetici", domain.RoleManager)
			_ = seedUser(ctx, db, siteID, "muhasebe@siteyonet.com", passStr, "Ayşe Muhasebe", domain.RoleAccountant)
			_ = seedUser(ctx, db, siteID, "denetim@siteyonet.com", passStr, "Kemal Denetçi", domain.RoleAuditor)
			_ = seedUser(ctx, db, siteID, "malik@siteyonet.com", passStr, "Hasan Malik", domain.RoleOwner)
			_ = seedUser(ctx, db, siteID, "kiraci@siteyonet.com", passStr, "Can Kiracı", domain.RoleTenant)
		}

		seedTotal := int64(149 * 150000)
		if os.Getenv("FULL_SIMULATION") == "false" {
			seedTotal = 19 * 150000
		}
		periods := map[string]uuid.UUID{"2026": periodID}
		for year := 2023; year < 2026; year++ {
			var id uuid.UUID
			err := db.Executor(ctx).QueryRow(ctx, `INSERT INTO "fy-fiscal-periods" (site_id,name,start_date,end_date,status) VALUES ($1,$2,$3::date,$4::date,'OPEN') RETURNING id`, siteID, fmt.Sprintf("%d Mali Yılı", year), fmt.Sprintf("%d-01-01", year), fmt.Sprintf("%d-12-31", year)).Scan(&id)
			if err != nil {
				log.Fatal(err)
			}
			periods[strconv.Itoa(year)] = id
		}
		// Create standard dues definition & accruals for this site
		var duesDefID uuid.UUID
		_ = db.Executor(ctx).QueryRow(ctx, `
			INSERT INTO "fy-dues-definitions" (site_id, name, total_amount_cents, distribution_type, due_day, late_fee_rate)
			VALUES ($1, 'Genel İşletme Aidatı', $2, 'EQUAL', 15, 5.00)
			RETURNING id
		`, siteID, seedTotal).Scan(&duesDefID)

		// Create 3 historical accruals (2026-08, 2026-09, 2026-10)
		months := simulationMonths()
		accrualIDs := make(map[string]uuid.UUID)
		for _, m := range months {
			var accID uuid.UUID
			_ = db.Executor(ctx).QueryRow(ctx, `
				INSERT INTO "fy-accruals" (site_id, dues_definition_id, period_id, title, term_month, distribution_type, total_amount_cents, created_by)
				VALUES ($1, $2, $3, $4, $5, 'EQUAL', $7, $6)
				RETURNING id
			`, siteID, duesDefID, periods[m[:4]], m+" Dönemi İşletme Aidatı", m, managerID, seedTotal).Scan(&accID)
			accrualIDs[m] = accID
		}

		// Create Blocks and Units
		blocks := []string{"A Blok", "B Blok"}
		unitsPerBlock := 75
		if os.Getenv("FULL_SIMULATION") == "false" {
			unitsPerBlock = 10 // 40 sites * 2 blocks * 75 units = 6,000 units!
		}

		residents := []struct {
			name  string
			email string
			role  string
		}{
			{"Ahmet Yılmaz", "ahmet.yilmaz@siteyonet.com", domain.RoleOwner},
			{"Mehmet Demir", "mehmet.demir@siteyonet.com", domain.RoleTenant},
			{"Ayşe Kaya", "ayse.kaya@siteyonet.com", domain.RoleOwner},
			{"Fatma Çelik", "fatma.celik@siteyonet.com", domain.RoleTenant},
			{"Mustafa Şahin", "mustafa.sahin@siteyonet.com", domain.RoleOwner},
			{"Zeynep Öztürk", "zeynep.ozturk@siteyonet.com", domain.RoleTenant},
			{"Emre Aydın", "emre.aydin@siteyonet.com", domain.RoleOwner},
			{"Selin Arslan", "selin.arslan@siteyonet.com", domain.RoleTenant},
			{"Burak Doğan", "burak.dogan@siteyonet.com", domain.RoleOwner},
			{"Gamze Kılıç", "gamze.kilic@siteyonet.com", domain.RoleTenant},
		}

		for _, bName := range blocks {
			var blockID uuid.UUID
			_ = db.Executor(ctx).QueryRow(ctx, `
				INSERT INTO "fy-blocks" (site_id, name, total_floors)
				VALUES ($1, $2, 8)
				ON CONFLICT (site_id, name) DO UPDATE SET total_floors = EXCLUDED.total_floors
				RETURNING id
			`, siteID, bName).Scan(&blockID)

			for uNum := 1; uNum <= unitsPerBlock; uNum++ {
				uNoStr := strconv.Itoa(uNum)
				isExempt := (uNum == 1 && bName == "A Blok") // Unit 1 A block is exempt doorman flat

				var oID, tID *uuid.UUID
				var residentUser uuid.UUID
				if !isExempt {
					resIdx := (uNum - 1) % len(residents)
					res := residents[resIdx]
					resID := seedUser(ctx, db, siteID, fmt.Sprintf("%s_u%d_%s", prefix, uNum, res.email), passStr, res.name, res.role)
					residentUser = resID

					if res.role == domain.RoleTenant {
						oID = &ownerID
						tID = &resID
					} else {
						oID = &resID
					}
				} else {
					residentUser = ownerID
				}

				if i == 0 && bName == "A Blok" && uNum == 2 {
					own := seedUser(ctx, db, siteID, "malik@siteyonet.com", passStr, "Hasan Malik", domain.RoleOwner)
					ten := seedUser(ctx, db, siteID, "kiraci@siteyonet.com", passStr, "Can Kiracı", domain.RoleTenant)
					oID, tID, residentUser = &own, &ten, ten
				}
				var unitID uuid.UUID
				_ = db.Executor(ctx).QueryRow(ctx, `
					INSERT INTO "fy-units" (site_id, block_id, unit_number, floor_number, land_share, gross_area_m2, resident_count, is_exempt, owner_id, tenant_id)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
					ON CONFLICT (block_id, unit_number) DO UPDATE SET land_share = EXCLUDED.land_share
					RETURNING id
				`, siteID, blockID, uNoStr, (uNum/4)+1, 100+r.Intn(50), 100.0+float64(r.Intn(40)), 1+r.Intn(4), isExempt, oID, tID).Scan(&unitID)

				// Seed past debts and payments linked to accruals
				if !isExempt && unitID != uuid.Nil {
					seedUnitDebtsAndPayments(ctx, db, siteID, unitID, residentUser, managerID, accrualIDs, r)
				}
			}
		}

		flushHistory(ctx)
		// Seed Real-world Site Expenses
		seedExpenses(ctx, db, siteID, periodID, managerID)

		for category, amount := range map[string]int64{"ENERGY": 1500000, "WATER": 800000, "ELEVATOR": 1200000, "CLEANING": 2000000, "SECURITY": 3500000, "LANDSCAPING": 1000000} {
			if _, err := db.Executor(ctx).Exec(ctx, `INSERT INTO "fy-budgets" (site_id,category,amount_cents) VALUES ($1,$2,$3)`, siteID, category, amount); err != nil {
				log.Fatal(err)
			}
		}
		seedLedger(ctx, db, siteID, managerID)

		// Create Sample Bank Statements (with real-world fuzzy and messy descriptions)
		seedBankStatements(ctx, db, siteID, r)

		// Create Sample Announcements & Decisions
		seedAnnouncementsAndDecisions(ctx, db, siteID, managerID)
	}

	if _, err := db.Executor(ctx).Exec(ctx, `INSERT INTO "fy-user-sites" (user_id,site_id,role) SELECT id,site_id,role FROM "fy-ox-users" WHERE site_id IS NOT NULL AND role!='superadmin' ON CONFLICT DO NOTHING`); err != nil {
		log.Fatal(err)
	}
	if _, err := db.Executor(ctx).Exec(ctx, `INSERT INTO "fy-user-sites" (user_id,site_id,role) SELECT u.id,s.id,'manager' FROM "fy-ox-users" u CROSS JOIN "fy-sites" s WHERE u.email='yonetici@siteyonet.com' ON CONFLICT DO NOTHING`); err != nil {
		log.Fatal(err)
	}
	if err := ctx.Err(); err != nil {
		log.Fatal(err)
	}
	var sites, units, debts int
	if err := db.Executor(ctx).QueryRow(ctx, `SELECT (SELECT count(*) FROM "fy-sites"),(SELECT count(*) FROM "fy-units"),(SELECT count(*) FROM "fy-debts")`).Scan(&sites, &units, &debts); err != nil {
		log.Fatal(err)
	}
	expectedUnits, expectedDebts := 6000, 214560
	if os.Getenv("FULL_SIMULATION") == "false" {
		expectedUnits, expectedDebts = 100, 285
	}
	if sites != siteCount || units != expectedUnits || debts != expectedDebts {
		log.Fatalf("Seed incomplete: sites=%d units=%d debts=%d", sites, units, debts)
	}
	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `ANALYZE "fy-debts"; ANALYZE "fy-units"; ANALYZE "fy-payments"; ANALYZE "fy-journal-lines"`); err != nil {
		log.Fatal(err)
	}
	log.Printf("Seed completed: sites=%d units=%d debts=%d", sites, units, debts)
}

func seedUser(ctx context.Context, db *repository.DB, siteID uuid.UUID, email, pass, name, role string) uuid.UUID {
	var userID uuid.UUID
	_ = db.Executor(ctx).QueryRow(ctx, `
		INSERT INTO "fy-ox-users" (site_id, email, password_hash, full_name, role, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
		ON CONFLICT (email) DO UPDATE SET full_name = EXCLUDED.full_name
		RETURNING id
	`, siteID, email, pass, name, role).Scan(&userID)
	return userID
}

func seedAccounts(ctx context.Context, db *repository.DB, siteID uuid.UUID) {
	accounts := []struct {
		code, name, aType string
	}{
		{"100", "Kasa Hesabı", domain.AccountAsset},
		{"102", "Banka Hesabı (Garanti BBVA)", domain.AccountAsset},
		{"120", "Aidat Alacakları (Daireler)", domain.AccountAsset},
		{"320", "Satıcılar (Tedarikçi Borçları)", domain.AccountLiability},
		{"340", "Alınan Avanslar (Fazla Ödemeler)", domain.AccountLiability},
		{"600", "Aidat Gelirleri", domain.AccountRevenue},
		{"642", "Gecikme Zammı Gelirleri", domain.AccountRevenue},
		{"770", "Genel Yönetim Giderleri", domain.AccountExpense},
	}

	for _, a := range accounts {
		_, _ = db.Executor(ctx).Exec(ctx, `
			INSERT INTO "fy-ledger-accounts" (site_id, code, name, account_type)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (site_id, code) DO NOTHING
		`, siteID, a.code, a.name, a.aType)
	}
}

var debtRows, paymentRows, allocationRows [][]interface{}

func seedUnitDebtsAndPayments(ctx context.Context, db *repository.DB, siteID, unitID, userID, managerID uuid.UUID, accrualIDs map[string]uuid.UUID, r *rand.Rand) {
	for i, m := range simulationMonths() {
		month, err := time.Parse("2006-01", m)
		if err != nil {
			log.Fatal(err)
		}
		dueDate := month.AddDate(0, 0, 14)
		principal := int64(150000)
		debtID := uuid.New()
		paid := int64(0)
		status := "UNPAID"
		if i < 2 && r.Float64() > 0.3 {
			paymentID := uuid.New()
			payDate := dueDate.AddDate(0, 0, r.Intn(10)-5)
			paymentRows = append(paymentRows, []interface{}{paymentID, siteID, unitID, userID, payDate, principal, "BANK_TRANSFER", "Aidat Havalesi", managerID})
			allocationRows = append(allocationRows, []interface{}{paymentID, debtID, principal})
			paid, status = principal, "PAID"
		}
		debtRows = append(debtRows, []interface{}{debtID, siteID, unitID, userID, accrualIDs[m], "Aylık İşletme Aidatı", "DUES", m, principal, paid, dueDate, status})
	}
}
func flushHistory(ctx context.Context) {
	tx := ctx.Value(middleware.TransactionKey).(pgx.Tx)
	for _, batch := range []struct {
		table   string
		columns []string
		rows    [][]interface{}
	}{
		{"fy-debts", []string{"id", "site_id", "unit_id", "user_id", "accrual_id", "title", "debt_type", "term_month", "principal_cents", "paid_principal_cents", "due_date", "status"}, debtRows},
		{"fy-payments", []string{"id", "site_id", "unit_id", "user_id", "payment_date", "amount_cents", "payment_channel", "description", "created_by"}, paymentRows},
		{"fy-payment-allocations", []string{"payment_id", "debt_id", "allocated_principal_cents"}, allocationRows},
	} {
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{batch.table}, batch.columns, pgx.CopyFromRows(batch.rows)); err != nil {
			log.Fatal(err)
		}
	}
	debtRows, paymentRows, allocationRows = nil, nil, nil
}

func seedExpenses(ctx context.Context, db *repository.DB, siteID, periodID, managerID uuid.UUID) {
	expenses := []struct {
		category string
		title    string
		vendor   string
		invNo    string
		amount   int64
		date     time.Time
	}{
		{"ELEVATOR", "KONE Asansör Aylık Periyodik Bakım Sözleşmesi", "KONE Asansör San. A.Ş.", "FAT-2026-0881", 850000, time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)},
		{"ENERGY", "Ortak Alan & Çevre Aydınlatma Elektrik Faturası", "Enerjisa Dağıtım A.Ş.", "FAT-2026-1044", 680000, time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)},
		{"SECURITY", "Güvenlik & Danışma Personeli Aylık Hizmet Bedeli", "Securitas Güvenlik Hizmetleri", "FAT-2026-1209", 1400000, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)},
		{"CLEANING", "Ortak Alan Temizlik Malzemeleri ve Hijyen Ürünleri", "Eczacıbaşı Profesyonel", "FAT-2026-1450", 310000, time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)},
		{"LANDSCAPING", "Bahçe Bakım, Çim Biçme ve Kışlık Peyzaj İlaçlama", "Yeşil Vadi Peyzaj Ltd.", "FAT-2026-1512", 520000, time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)},
	}

	for _, e := range expenses {
		_, _ = db.Executor(ctx).Exec(ctx, `
			INSERT INTO "fy-expenses" (site_id, period_id, category, title, vendor_name, invoice_number, invoice_date, amount_cents, paid_cents, status, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, 'PAID', $9)
		`, siteID, periodID, e.category, e.title, e.vendor, e.invNo, e.date, e.amount, managerID)
	}
}

func seedBankStatements(ctx context.Context, db *repository.DB, siteID uuid.UUID, r *rand.Rand) {
	var csvText strings.Builder
	writer := csv.NewWriter(&csvText)
	_ = writer.Write([]string{"date", "reference", "description", "amount_cents"})
	descriptions := []string{
		"A BLOK DAIRE: 5 AHMET YILMAZ AIDAT",
		"B BLOK NO 12 AIDAT HAVALESI MEHMET DEMIR",
		"KIRA/AIDAT ODEMESI D:8 A BLOK",
		"1500 TL SITE AIDAT HAVALE REF99234 DAIRE 3",
		"HATALI ACIKLAMA - FATURA BEDELI AHMET B-7",
		"GARANTI BBVA EFT D.14 A-BLOK",
	}

	for i, desc := range descriptions {
		amt := int64(150000)
		sDate := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -r.Intn(30))
		ref := fmt.Sprintf("EFT-%06d", 100000+i)
		_ = writer.Write([]string{sDate.Format("2006-01-02"), ref, desc, strconv.FormatInt(amt, 10)})

		_, _ = db.Executor(ctx).Exec(ctx, `
			INSERT INTO "fy-bank-statements" (site_id, statement_date, reference_number, raw_description, amount_cents, match_status)
			VALUES ($1, $2, $3, $4, $5, 'UNMATCHED')
		`, siteID, sDate, ref, desc, amt)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Fatal(err)
	}
	var code string
	if err := db.Executor(ctx).QueryRow(ctx, `SELECT code FROM "fy-sites" WHERE id=$1`, siteID).Scan(&code); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll("bank-statements", 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("bank-statements", code+".csv"), []byte(csvText.String()), 0644); err != nil {
		log.Fatal(err)
	}
}

func seedAnnouncementsAndDecisions(ctx context.Context, db *repository.DB, siteID, managerID uuid.UUID) {
	// Announcements
	_, _ = db.Executor(ctx).Exec(ctx, `
		INSERT INTO "fy-announcements" (site_id, title, content, priority, created_by)
		VALUES ($1, 'Yıllık Olağan Genel Kurul Toplantısı', 'Sitemizin yıllık olağan genel kurul toplantısı 15 Kasım Pazar günü saat 14:00te site lokalinde yapılacaktır.', 'HIGH', $2),
		       ($1, 'Kazan Dairesi Periyodik Bakımı', 'Kış sezonu öncesi merkezi ısıtma kazanlarının bakımı 28 Ekim Çarşamba günü yapılacaktır.', 'NORMAL', $2)
	`, siteID, managerID)

	// Decisions
	_, _ = db.Executor(ctx).Exec(ctx, `
		INSERT INTO "fy-decisions" (site_id, decision_number, decision_date, title, content, vote_summary, created_by)
		VALUES ($1, '2026/01', '2026-01-10', 'Güvenlik Kamera Sistemi Yenilenmesi', 'Blok girişleri ve çevre güvenlik kameralarının IP kamera sistemi ile yenilenmesine karar verilmiştir.', 'Oy birliği ile kabul edildi', $2),
		       ($1, '2026/02', '2026-03-15', 'Asansör Revizyonu ve Yeşil Etiket Alımı', 'Mavi Vadi A Blok asansörünün TSE standartlarına uygun revizyonunun yapılmasına karar verilmiştir.', 'Oy çokluğu ile kabul edildi', $2)
	`, siteID, managerID)
}

func simulationMonths() []string {
	if os.Getenv("FULL_SIMULATION") == "false" {
		return []string{"2026-08", "2026-09", "2026-10"}
	}
	months := make([]string, 36)
	start := time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC)
	for i := range months {
		months[i] = start.AddDate(0, i, 0).Format("2006-01")
	}
	return months
}

func seedLedger(ctx context.Context, db *repository.DB, site, manager uuid.UUID) {
	queries := []string{
		`WITH entries AS (
 INSERT INTO "fy-journal-entries" (site_id,period_id,entry_number,entry_date,description,created_by)
 SELECT site_id,period_id,replace(term_month,'-','')::bigint,(term_month||'-01')::date,'Seed accrual '||term_month,$2 FROM "fy-accruals" WHERE site_id=$1
 RETURNING id,site_id,description)
 INSERT INTO "fy-journal-lines" (journal_entry_id,account_id,debit_cents,credit_cents,description)
 SELECT e.id,a.id,CASE WHEN a.code='120' THEN ac.total_amount_cents ELSE 0 END,CASE WHEN a.code='600' THEN ac.total_amount_cents ELSE 0 END,e.description
 FROM entries e JOIN "fy-accruals" ac ON ac.site_id=e.site_id AND e.description='Seed accrual '||ac.term_month JOIN "fy-ledger-accounts" a ON a.site_id=e.site_id AND a.code IN ('120','600')`,
		`WITH entries AS (
 INSERT INTO "fy-journal-entries" (site_id,period_id,entry_number,entry_date,description,created_by)
 SELECT p.site_id,f.id,1000000+row_number() OVER (ORDER BY p.id),p.payment_date,'Seed payment '||p.id::text,$2
 FROM "fy-payments" p JOIN "fy-fiscal-periods" f ON f.site_id=p.site_id AND p.payment_date BETWEEN f.start_date AND f.end_date WHERE p.site_id=$1
 RETURNING id,site_id,description)
 INSERT INTO "fy-journal-lines" (journal_entry_id,account_id,debit_cents,credit_cents,description)
 SELECT e.id,a.id,CASE WHEN a.code='102' THEN p.amount_cents ELSE 0 END,CASE WHEN a.code='120' THEN p.amount_cents ELSE 0 END,e.description
 FROM entries e JOIN "fy-payments" p ON p.site_id=e.site_id AND e.description='Seed payment '||p.id::text JOIN "fy-ledger-accounts" a ON a.site_id=e.site_id AND a.code IN ('102','120')`,
		`WITH entries AS (
 INSERT INTO "fy-journal-entries" (site_id,period_id,entry_number,entry_date,description,created_by)
 SELECT site_id,period_id,2000000+row_number() OVER (ORDER BY id),invoice_date,'Seed expense '||id::text,$2 FROM "fy-expenses" WHERE site_id=$1
 RETURNING id,site_id,description)
 INSERT INTO "fy-journal-lines" (journal_entry_id,account_id,debit_cents,credit_cents,description)
 SELECT e.id,a.id,CASE WHEN a.code='770' THEN x.amount_cents ELSE 0 END,CASE WHEN a.code='102' THEN x.amount_cents ELSE 0 END,e.description
 FROM entries e JOIN "fy-expenses" x ON x.site_id=e.site_id AND e.description='Seed expense '||x.id::text JOIN "fy-ledger-accounts" a ON a.site_id=e.site_id AND a.code IN ('770','102')`,
	}
	for _, q := range queries {
		if _, err := db.Executor(ctx).Exec(ctx, q, site, manager); err != nil {
			log.Fatal(err)
		}
	}
	if _, err := db.Executor(ctx).Exec(ctx, `UPDATE "fy-accruals" a SET journal_entry_id=e.id FROM "fy-journal-entries" e WHERE a.site_id=$1 AND e.site_id=a.site_id AND e.description='Seed accrual '||a.term_month`, site); err != nil {
		log.Fatal(err)
	}

}
