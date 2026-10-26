package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/siteyonet/siteyonet/internal/config"
	"github.com/siteyonet/siteyonet/internal/domain"
	"github.com/siteyonet/siteyonet/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

func main() {
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

	// Bypass RLS during initial seed
	_, _ = db.Pool.Exec(ctx, "SELECT set_config('app.bypass_rls', 'true', false)")

	// 1. Check if seed already done
	var existingUsersCount int
	_ = db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM "fy-ox-users"`).Scan(&existingUsersCount)
	if existingUsersCount > 0 {
		if os.Getenv("FORCE_SEED") != "true" {
			log.Printf("Database already contains %d users. Skipping re-seed. (Set FORCE_SEED=true to override)", existingUsersCount)
			return
		}
		log.Println("FORCE_SEED=true: Truncating existing data for clean re-seed...")
		_, _ = db.Pool.Exec(ctx, `
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
	siteCount := 5
	if os.Getenv("FULL_SIMULATION") == "true" {
		siteCount = 40
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
		err := db.Pool.QueryRow(ctx, `
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
		_ = db.Pool.QueryRow(ctx, `
			INSERT INTO "fy-fiscal-periods" (site_id, name, start_date, end_date, status)
			VALUES ($1, '2026 Mali Yılı', '2026-01-01', '2026-12-31', 'OPEN')
			ON CONFLICT (site_id, start_date, end_date) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		`, siteID).Scan(&periodID)

		// Create standard role demo accounts on primary site (Site 1)
		var managerID, ownerID uuid.UUID
		prefix := fmt.Sprintf("site%d", i+1)

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

		// Create standard dues definition & accruals for this site
		var duesDefID uuid.UUID
		_ = db.Pool.QueryRow(ctx, `
			INSERT INTO "fy-dues-definitions" (site_id, name, total_amount_cents, distribution_type, due_day, late_fee_rate)
			VALUES ($1, 'Genel İşletme Aidatı', 2850000, 'LAND_SHARE', 15, 5.00)
			RETURNING id
		`, siteID).Scan(&duesDefID)

		// Create 3 historical accruals (2026-08, 2026-09, 2026-10)
		months := []string{"2026-08", "2026-09", "2026-10"}
		accrualIDs := make(map[string]uuid.UUID)
		for _, m := range months {
			var accID uuid.UUID
			_ = db.Pool.QueryRow(ctx, `
				INSERT INTO "fy-accruals" (site_id, dues_definition_id, period_id, title, term_month, distribution_type, total_amount_cents, created_by)
				VALUES ($1, $2, $3, $4, $5, 'LAND_SHARE', 2850000, $6)
				RETURNING id
			`, siteID, duesDefID, periodID, m+" Dönemi İşletme Aidatı", m, managerID).Scan(&accID)
			accrualIDs[m] = accID
		}

		// Create Blocks and Units
		blocks := []string{"A Blok", "B Blok"}
		unitsPerBlock := 10
		if os.Getenv("FULL_SIMULATION") == "true" {
			unitsPerBlock = 75 // 40 sites * 2 blocks * 75 units = 6,000 units!
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
			_ = db.Pool.QueryRow(ctx, `
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

				var unitID uuid.UUID
				_ = db.Pool.QueryRow(ctx, `
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

		// Seed Real-world Site Expenses
		seedExpenses(ctx, db, siteID, periodID, managerID)

		// Create Sample Bank Statements (with real-world fuzzy and messy descriptions)
		seedBankStatements(ctx, db, siteID, r)

		// Create Sample Announcements & Decisions
		seedAnnouncementsAndDecisions(ctx, db, siteID, managerID)
	}

	log.Println("Seed process completed successfully!")
}

func seedUser(ctx context.Context, db *repository.DB, siteID uuid.UUID, email, pass, name, role string) uuid.UUID {
	var userID uuid.UUID
	_ = db.Pool.QueryRow(ctx, `
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
		{"770", "Genel Yönetim Giderleri", domain.AccountExpense},
	}

	for _, a := range accounts {
		_, _ = db.Pool.Exec(ctx, `
			INSERT INTO "fy-ledger-accounts" (site_id, code, name, account_type)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (site_id, code) DO NOTHING
		`, siteID, a.code, a.name, a.aType)
	}
}

func seedUnitDebtsAndPayments(ctx context.Context, db *repository.DB, siteID, unitID, userID, managerID uuid.UUID, accrualIDs map[string]uuid.UUID, r *rand.Rand) {
	months := []string{"2026-08", "2026-09", "2026-10"}
	for i, m := range months {
		dueDay := 15
		dueDate := time.Date(2026, time.Month(8+i), dueDay, 0, 0, 0, 0, time.UTC)
		principal := int64(150000) // 1.500,00 TL
		accID := accrualIDs[m]

		var debtID uuid.UUID
		_ = db.Pool.QueryRow(ctx, `
			INSERT INTO "fy-debts" (site_id, unit_id, user_id, accrual_id, title, debt_type, term_month, principal_cents, due_date, status)
			VALUES ($1, $2, $3, $4, 'Aylık İşletme Aidatı', 'DUES', $5, $6, $7, 'UNPAID')
			RETURNING id
		`, siteID, unitID, userID, accID, m, principal, dueDate).Scan(&debtID)

		// Simulate payment for older months
		if i < 2 && r.Float64() > 0.3 {
			payDate := dueDate.AddDate(0, 0, r.Intn(10)-5)
			var paymentID uuid.UUID
			_ = db.Pool.QueryRow(ctx, `
				INSERT INTO "fy-payments" (site_id, unit_id, user_id, payment_date, amount_cents, payment_channel, description, created_by)
				VALUES ($1, $2, $3, $4, $5, 'BANK_TRANSFER', 'Aidat Havalesi', $6)
				RETURNING id
			`, siteID, unitID, userID, payDate, principal, managerID).Scan(&paymentID)

			_, _ = db.Pool.Exec(ctx, `
				UPDATE "fy-debts" 
				SET paid_principal_cents = $1, status = 'PAID', updated_at = CURRENT_TIMESTAMP 
				WHERE id = $2
			`, principal, debtID)

			_, _ = db.Pool.Exec(ctx, `
				INSERT INTO "fy-payment-allocations" (payment_id, debt_id, allocated_principal_cents)
				VALUES ($1, $2, $3)
			`, paymentID, debtID, principal)
		}
	}
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
		_, _ = db.Pool.Exec(ctx, `
			INSERT INTO "fy-expenses" (site_id, period_id, category, title, vendor_name, invoice_number, invoice_date, amount_cents, paid_cents, status, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, 'PAID', $9)
		`, siteID, periodID, e.category, e.title, e.vendor, e.invNo, e.date, e.amount, managerID)
	}
}

func seedBankStatements(ctx context.Context, db *repository.DB, siteID uuid.UUID, r *rand.Rand) {
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
		sDate := time.Now().AddDate(0, 0, -r.Intn(30))
		ref := fmt.Sprintf("EFT-%06d", 100000+i)

		_, _ = db.Pool.Exec(ctx, `
			INSERT INTO "fy-bank-statements" (site_id, statement_date, reference_number, raw_description, amount_cents, match_status)
			VALUES ($1, $2, $3, $4, $5, 'UNMATCHED')
		`, siteID, sDate, ref, desc, amt)
	}
}

func seedAnnouncementsAndDecisions(ctx context.Context, db *repository.DB, siteID, managerID uuid.UUID) {
	// Announcements
	_, _ = db.Pool.Exec(ctx, `
		INSERT INTO "fy-announcements" (site_id, title, content, priority, created_by)
		VALUES ($1, 'Yıllık Olağan Genel Kurul Toplantısı', 'Sitemizin yıllık olağan genel kurul toplantısı 15 Kasım Pazar günü saat 14:00te site lokalinde yapılacaktır.', 'HIGH', $2),
		       ($1, 'Kazan Dairesi Periyodik Bakımı', 'Kış sezonu öncesi merkezi ısıtma kazanlarının bakımı 28 Ekim Çarşamba günü yapılacaktır.', 'NORMAL', $2)
	`, siteID, managerID)

	// Decisions
	_, _ = db.Pool.Exec(ctx, `
		INSERT INTO "fy-decisions" (site_id, decision_number, decision_date, title, content, vote_summary, created_by)
		VALUES ($1, '2026/01', '2026-01-10', 'Güvenlik Kamera Sistemi Yenilenmesi', 'Blok girişleri ve çevre güvenlik kameralarının IP kamera sistemi ile yenilenmesine karar verilmiştir.', 'Oy birliği ile kabul edildi', $2),
		       ($1, '2026/02', '2026-03-15', 'Asansör Revizyonu ve Yeşil Etiket Alımı', 'Mavi Vadi A Blok asansörünün TSE standartlarına uygun revizyonunun yapılmasına karar verilmiştir.', 'Oy çokluğu ile kabul edildi', $2)
	`, siteID, managerID)
}
