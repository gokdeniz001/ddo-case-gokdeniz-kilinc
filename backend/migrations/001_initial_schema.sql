-- PostgreSQL 16 Migration: Initial Schema
-- All table names must start with "fy-"
-- The users table must be named exactly "fy-ox-users"

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- 1. Users table (Strictly named "fy-ox-users")
CREATE TABLE IF NOT EXISTS "fy-ox-users" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID, -- NULL for platform superadmins, or primary site
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    phone VARCHAR(50),
    role VARCHAR(50) NOT NULL CHECK (role IN ('superadmin', 'manager', 'accountant', 'auditor', 'owner', 'tenant')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 2. Sites (Tenants)
CREATE TABLE IF NOT EXISTS "fy-sites" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50) NOT NULL UNIQUE,
    address TEXT,
    city VARCHAR(100) NOT NULL DEFAULT 'İstanbul',
    district VARCHAR(100),
    bank_name VARCHAR(100),
    bank_iban VARCHAR(50),
    default_late_fee_rate NUMERIC(5, 2) NOT NULL DEFAULT 5.00, -- monthly %5 default delay penalty
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Add foreign key from users to sites
ALTER TABLE "fy-ox-users" 
    ADD CONSTRAINT fk_users_site FOREIGN KEY (site_id) REFERENCES "fy-sites"(id) ON DELETE SET NULL;

-- User-Site mapping for users managing/auditing multiple sites
CREATE TABLE IF NOT EXISTS "fy-user-sites" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "fy-ox-users"(id) ON DELETE CASCADE,
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL CHECK (role IN ('manager', 'accountant', 'auditor', 'owner', 'tenant')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, site_id)
);

-- 3. Blocks
CREATE TABLE IF NOT EXISTS "fy-blocks" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    total_floors INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, name)
);

-- 4. Units (Daireler)
CREATE TABLE IF NOT EXISTS "fy-units" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    block_id UUID NOT NULL REFERENCES "fy-blocks"(id) ON DELETE CASCADE,
    unit_number VARCHAR(50) NOT NULL,
    floor_number INT NOT NULL DEFAULT 0,
    land_share INT NOT NULL DEFAULT 100,      -- Arsa payı (örn. 100/10000)
    gross_area_m2 NUMERIC(8, 2) NOT NULL DEFAULT 100.00, -- m2
    resident_count INT NOT NULL DEFAULT 1,    -- Kişi sayısı
    is_exempt BOOLEAN NOT NULL DEFAULT false, -- Muaf daire (kapıcı dairesi vb.)
    owner_id UUID REFERENCES "fy-ox-users"(id) ON DELETE SET NULL,
    tenant_id UUID REFERENCES "fy-ox-users"(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(block_id, unit_number)
);

-- Ownership / Tenancy history for mid-period transfers
CREATE TABLE IF NOT EXISTS "fy-unit-occupancies" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unit_id UUID NOT NULL REFERENCES "fy-units"(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES "fy-ox-users"(id) ON DELETE CASCADE,
    occupancy_type VARCHAR(20) NOT NULL CHECK (occupancy_type IN ('owner', 'tenant')),
    start_date DATE NOT NULL,
    end_date DATE, -- NULL if currently active
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 5. Fiscal Periods (Muhasebe Dönemleri)
CREATE TABLE IF NOT EXISTS "fy-fiscal-periods" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'CLOSED', 'LOCKED')),
    closed_at TIMESTAMPTZ,
    closed_by UUID REFERENCES "fy-ox-users"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, start_date, end_date)
);

-- 6. Dues Definitions (Aidat Tanımları)
CREATE TABLE IF NOT EXISTS "fy-dues-definitions" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    total_amount_cents BIGINT NOT NULL CHECK (total_amount_cents >= 0),
    distribution_type VARCHAR(50) NOT NULL CHECK (distribution_type IN ('EQUAL', 'LAND_SHARE', 'AREA_M2', 'RESIDENT_COUNT')),
    due_day INT NOT NULL DEFAULT 15,
    late_fee_rate NUMERIC(5, 2) NOT NULL DEFAULT 5.00,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 7. Accruals (Tahakkuklar - Dönemsel)
CREATE TABLE IF NOT EXISTS "fy-accruals" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    dues_definition_id UUID REFERENCES "fy-dues-definitions"(id) ON DELETE SET NULL,
    period_id UUID REFERENCES "fy-fiscal-periods"(id) ON DELETE RESTRICT,
    title VARCHAR(255) NOT NULL,
    term_month VARCHAR(7) NOT NULL, -- YYYY-MM
    distribution_type VARCHAR(50) NOT NULL,
    total_amount_cents BIGINT NOT NULL,
    is_reversed BOOLEAN NOT NULL DEFAULT false,
    reversal_accrual_id UUID REFERENCES "fy-accruals"(id),
    created_by UUID REFERENCES "fy-ox-users"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 8. Debts (Daire Borç Kalemleri)
CREATE TABLE IF NOT EXISTS "fy-debts" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    unit_id UUID NOT NULL REFERENCES "fy-units"(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES "fy-ox-users"(id) ON DELETE CASCADE,
    accrual_id UUID REFERENCES "fy-accruals"(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    debt_type VARCHAR(50) NOT NULL DEFAULT 'DUES', -- DUES, EXPENSE_SHARE, REPAIR, PENALTY, RESTRUCTURE
    term_month VARCHAR(7) NOT NULL, -- YYYY-MM
    principal_cents BIGINT NOT NULL CHECK (principal_cents >= 0),
    paid_principal_cents BIGINT NOT NULL DEFAULT 0 CHECK (paid_principal_cents <= principal_cents),
    paid_interest_cents BIGINT NOT NULL DEFAULT 0,
    due_date DATE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'UNPAID' CHECK (status IN ('UNPAID', 'PARTIAL', 'PAID', 'CANCELLED', 'RESTRUCTURED')),
    restructure_plan_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 9. Payments (Tahsilatlar)
CREATE TABLE IF NOT EXISTS "fy-payments" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    unit_id UUID NOT NULL REFERENCES "fy-units"(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES "fy-ox-users"(id) ON DELETE CASCADE,
    payment_date DATE NOT NULL,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    unallocated_cents BIGINT NOT NULL DEFAULT 0 CHECK (unallocated_cents >= 0), -- Fazla ödeme / Avans alacak
    payment_channel VARCHAR(50) NOT NULL DEFAULT 'BANK_TRANSFER' CHECK (payment_channel IN ('BANK_TRANSFER', 'CASH', 'CREDIT_CARD')),
    bank_reference VARCHAR(255),
    description TEXT,
    idempotency_key VARCHAR(255) UNIQUE,
    created_by UUID REFERENCES "fy-ox-users"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 10. Payment Allocations (Ödeme Mahsup Detayları: Faiz -> Anapara -> FIFO)
CREATE TABLE IF NOT EXISTS "fy-payment-allocations" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL REFERENCES "fy-payments"(id) ON DELETE CASCADE,
    debt_id UUID NOT NULL REFERENCES "fy-debts"(id) ON DELETE CASCADE,
    allocated_principal_cents BIGINT NOT NULL DEFAULT 0 CHECK (allocated_principal_cents >= 0),
    allocated_interest_cents BIGINT NOT NULL DEFAULT 0 CHECK (allocated_interest_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 11. Debt Restructuring (Borç Yapılandırma)
CREATE TABLE IF NOT EXISTS "fy-debt-restructurings" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    unit_id UUID NOT NULL REFERENCES "fy-units"(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES "fy-ox-users"(id) ON DELETE CASCADE,
    total_restructured_cents BIGINT NOT NULL,
    installment_count INT NOT NULL CHECK (installment_count > 0),
    notes TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'COMPLETED', 'DEFAULTED')),
    created_by UUID REFERENCES "fy-ox-users"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 12. Expenses (Gider Yönetimi)
CREATE TABLE IF NOT EXISTS "fy-expenses" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    period_id UUID REFERENCES "fy-fiscal-periods"(id) ON DELETE RESTRICT,
    category VARCHAR(100) NOT NULL, -- ELEC, WATER, ELEVATOR, CLEANING, SECURITY, REPAIR, MGMT
    title VARCHAR(255) NOT NULL,
    vendor_name VARCHAR(255) NOT NULL,
    invoice_number VARCHAR(100),
    invoice_date DATE NOT NULL,
    due_date DATE,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    paid_cents BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'APPROVED' CHECK (status IN ('PENDING', 'APPROVED', 'PAID', 'CANCELLED')),
    document_url TEXT,
    created_by UUID REFERENCES "fy-ox-users"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 13. Double-entry Chart of Accounts (Muhasebe Hesap Planı)
CREATE TABLE IF NOT EXISTS "fy-ledger-accounts" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    code VARCHAR(50) NOT NULL, -- 100 Kasa, 102 Banka, 120 Alıcılar/Daire Borçları, 340 Alınan Avanslar, 600 Aidat Gelirleri, 770 Genel Yönetim Giderleri
    name VARCHAR(255) NOT NULL,
    account_type VARCHAR(50) NOT NULL CHECK (account_type IN ('ASSET', 'LIABILITY', 'EQUITY', 'REVENUE', 'EXPENSE')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, code)
);

-- 14. Journal Entries (Yevmiye Kayıtları)
CREATE TABLE IF NOT EXISTS "fy-journal-entries" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    period_id UUID NOT NULL REFERENCES "fy-fiscal-periods"(id) ON DELETE RESTRICT,
    entry_number BIGINT NOT NULL,
    entry_date DATE NOT NULL,
    description TEXT NOT NULL,
    is_reversal BOOLEAN NOT NULL DEFAULT false,
    reversed_entry_id UUID REFERENCES "fy-journal-entries"(id),
    created_by UUID REFERENCES "fy-ox-users"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, period_id, entry_number)
);

-- Journal Lines (Yevmiye Satırları - Çift taraflı: Debit = Credit)
CREATE TABLE IF NOT EXISTS "fy-journal-lines" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_entry_id UUID NOT NULL REFERENCES "fy-journal-entries"(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES "fy-ledger-accounts"(id) ON DELETE RESTRICT,
    debit_cents BIGINT NOT NULL DEFAULT 0 CHECK (debit_cents >= 0),
    credit_cents BIGINT NOT NULL DEFAULT 0 CHECK (credit_cents >= 0),
    description VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((debit_cents > 0 AND credit_cents = 0) OR (credit_cents > 0 AND debit_cents = 0))
);

-- 15. Bank Statements (Banka Ekstresi & Dekont İçe Aktarma)
CREATE TABLE IF NOT EXISTS "fy-bank-statements" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    statement_date DATE NOT NULL,
    reference_number VARCHAR(100),
    raw_description TEXT NOT NULL,
    amount_cents BIGINT NOT NULL,
    matched_unit_id UUID REFERENCES "fy-units"(id),
    match_confidence NUMERIC(5, 2), -- 0.00 to 1.00
    match_status VARCHAR(20) NOT NULL DEFAULT 'UNMATCHED' CHECK (match_status IN ('UNMATCHED', 'AUTO_MATCHED', 'MANUAL_MATCHED', 'PROCESSED')),
    payment_id UUID REFERENCES "fy-payments"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 16. Clearance Certificates ('Borcu Yoktur' Belgeleri)
CREATE TABLE IF NOT EXISTS "fy-clearance-certificates" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    unit_id UUID NOT NULL REFERENCES "fy-units"(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES "fy-ox-users"(id) ON DELETE CASCADE,
    certificate_number VARCHAR(64) NOT NULL UNIQUE,
    verification_code VARCHAR(32) NOT NULL UNIQUE,
    sha256_hash VARCHAR(64) NOT NULL,
    issued_date DATE NOT NULL DEFAULT CURRENT_DATE,
    valid_until DATE NOT NULL,
    debt_balance_at_issue_cents BIGINT NOT NULL DEFAULT 0,
    issued_by UUID REFERENCES "fy-ox-users"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 17. Announcements (Duyurular)
CREATE TABLE IF NOT EXISTS "fy-announcements" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    priority VARCHAR(20) NOT NULL DEFAULT 'NORMAL' CHECK (priority IN ('LOW', 'NORMAL', 'HIGH', 'URGENT')),
    is_published BOOLEAN NOT NULL DEFAULT true,
    created_by UUID REFERENCES "fy-ox-users"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 18. Decisions (Yönetim Kurulu Kararları)
CREATE TABLE IF NOT EXISTS "fy-decisions" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    decision_number VARCHAR(50) NOT NULL,
    decision_date DATE NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    vote_summary TEXT,
    created_by UUID REFERENCES "fy-ox-users"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, decision_number)
);

-- 19. Maintenance Requests (Bakım ve Arıza Talepleri)
CREATE TABLE IF NOT EXISTS "fy-maintenance-requests" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    unit_id UUID REFERENCES "fy-units"(id) ON DELETE SET NULL,
    user_id UUID NOT NULL REFERENCES "fy-ox-users"(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    category VARCHAR(50) NOT NULL DEFAULT 'GENERAL',
    status VARCHAR(20) NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'IN_PROGRESS', 'RESOLVED', 'CLOSED', 'REJECTED')),
    priority VARCHAR(20) NOT NULL DEFAULT 'MEDIUM' CHECK (priority IN ('LOW', 'MEDIUM', 'HIGH')),
    resolution_notes TEXT,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 20. Audit Logs (Eklemeli-yalnız / Append-only Denetim Tablosu)
CREATE TABLE IF NOT EXISTS "fy-audit-logs" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    user_id UUID REFERENCES "fy-ox-users"(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL, -- e.g. CREATE_ACCRUAL, MAKE_PAYMENT, CLOSE_PERIOD
    entity_table VARCHAR(100) NOT NULL,
    entity_id UUID NOT NULL,
    old_values JSONB,
    new_values JSONB,
    ip_address VARCHAR(50),
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 21. Idempotency Keys (Kritik POST İstekleri için)
CREATE TABLE IF NOT EXISTS "fy-idempotency-keys" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(255) NOT NULL,
    site_id UUID REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    user_id UUID REFERENCES "fy-ox-users"(id) ON DELETE CASCADE,
    request_hash VARCHAR(64) NOT NULL,
    response_code INT NOT NULL,
    response_body JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(key, site_id)
);

-- Indices for high performance & EXPLAIN ANALYZE compliance
CREATE INDEX IF NOT EXISTS idx_units_site_block ON "fy-units"(site_id, block_id);
CREATE INDEX IF NOT EXISTS idx_units_owner ON "fy-units"(owner_id);
CREATE INDEX IF NOT EXISTS idx_units_tenant ON "fy-units"(tenant_id);
CREATE INDEX IF NOT EXISTS idx_debts_unit_status ON "fy-debts"(unit_id, status);
CREATE INDEX IF NOT EXISTS idx_debts_site_term ON "fy-debts"(site_id, term_month);
CREATE INDEX IF NOT EXISTS idx_payments_unit ON "fy-payments"(unit_id);
CREATE INDEX IF NOT EXISTS idx_payments_site_date ON "fy-payments"(site_id, payment_date);
CREATE INDEX IF NOT EXISTS idx_journal_entries_period ON "fy-journal-entries"(site_id, period_id);
CREATE INDEX IF NOT EXISTS idx_journal_lines_account ON "fy-journal-lines"(account_id);
CREATE INDEX IF NOT EXISTS idx_bank_desc_trgm ON "fy-bank-statements" USING gin (raw_description gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_audit_site_created ON "fy-audit-logs"(site_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_maintenance_site_status ON "fy-maintenance-requests"(site_id, status);
