package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Role constants
const (
	RoleSuperAdmin = "superadmin"
	RoleManager    = "manager"
	RoleAccountant = "accountant"
	RoleAuditor    = "auditor"
	RoleOwner      = "owner"
	RoleTenant     = "tenant"
)

// Distribution types for dues
const (
	DistEqual         = "EQUAL"
	DistLandShare     = "LAND_SHARE"
	DistAreaM2        = "AREA_M2"
	DistResidentCount = "RESIDENT_COUNT"
)

// Debt types and statuses
const (
	DebtTypeDues         = "DUES"
	DebtTypeExpenseShare = "EXPENSE_SHARE"
	DebtTypeRepair       = "REPAIR"
	DebtTypePenalty      = "PENALTY"
	DebtTypeRestructure  = "RESTRUCTURE"

	DebtStatusUnpaid       = "UNPAID"
	DebtStatusPartial      = "PARTIAL"
	DebtStatusPaid         = "PAID"
	DebtStatusCancelled    = "CANCELLED"
	DebtStatusRestructured = "RESTRUCTURED"
)

// Period status
const (
	PeriodStatusOpen   = "OPEN"
	PeriodStatusClosed = "CLOSED"
	PeriodStatusLocked = "LOCKED"
)

// Account types
const (
	AccountAsset     = "ASSET"
	AccountLiability = "LIABILITY"
	AccountEquity    = "EQUITY"
	AccountRevenue   = "REVENUE"
	AccountExpense   = "EXPENSE"
)

// Helper for Cents to TL formatted string
func FormatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	lira := cents / 100
	krs := cents % 100
	return fmt.Sprintf("%s%d,%02d TL", sign, lira, krs)
}

// User represents fy-ox-users
type User struct {
	ID           uuid.UUID  `json:"id"`
	SiteID       *uuid.UUID `json:"site_id,omitempty"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	FullName     string     `json:"full_name"`
	Phone        string     `json:"phone,omitempty"`
	Role         string     `json:"role"`
	IsActive     bool       `json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Site represents fy-sites
type Site struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	Code               string    `json:"code"`
	Address            string    `json:"address,omitempty"`
	City               string    `json:"city"`
	District           string    `json:"district,omitempty"`
	BankName           string    `json:"bank_name,omitempty"`
	BankIBAN           string    `json:"bank_iban,omitempty"`
	DefaultLateFeeRate float64   `json:"default_late_fee_rate"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Block represents fy-blocks
type Block struct {
	ID          uuid.UUID `json:"id"`
	SiteID      uuid.UUID `json:"site_id"`
	Name        string    `json:"name"`
	TotalFloors int       `json:"total_floors"`
	CreatedAt   time.Time `json:"created_at"`
}

// Unit represents fy-units
type Unit struct {
	ID            uuid.UUID  `json:"id"`
	SiteID        uuid.UUID  `json:"site_id"`
	BlockID       uuid.UUID  `json:"block_id"`
	BlockName     string     `json:"block_name,omitempty"`
	UnitNumber    string     `json:"unit_number"`
	FloorNumber   int        `json:"floor_number"`
	LandShare     int        `json:"land_share"`
	GrossAreaM2   float64    `json:"gross_area_m2"`
	ResidentCount int        `json:"resident_count"`
	IsExempt      bool       `json:"is_exempt"`
	OwnerID       *uuid.UUID `json:"owner_id,omitempty"`
	OwnerName     string     `json:"owner_name,omitempty"`
	TenantID      *uuid.UUID `json:"tenant_id,omitempty"`
	TenantName    string     `json:"tenant_name,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// FiscalPeriod represents fy-fiscal-periods
type FiscalPeriod struct {
	ID        uuid.UUID  `json:"id"`
	SiteID    uuid.UUID  `json:"site_id"`
	Name      string     `json:"name"`
	StartDate time.Time  `json:"start_date"`
	EndDate   time.Time  `json:"end_date"`
	Status    string     `json:"status"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	ClosedBy  *uuid.UUID `json:"closed_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// DuesDefinition represents fy-dues-definitions
type DuesDefinition struct {
	ID               uuid.UUID `json:"id"`
	SiteID           uuid.UUID `json:"site_id"`
	Name             string    `json:"name"`
	TotalAmountCents int64     `json:"total_amount_cents"`
	DistributionType string    `json:"distribution_type"`
	DueDay           int       `json:"due_day"`
	LateFeeRate      float64   `json:"late_fee_rate"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
}

// Accrual represents fy-accruals
type Accrual struct {
	ID                uuid.UUID  `json:"id"`
	SiteID            uuid.UUID  `json:"site_id"`
	DuesDefinitionID  *uuid.UUID `json:"dues_definition_id,omitempty"`
	PeriodID          uuid.UUID  `json:"period_id"`
	Title             string     `json:"title"`
	TermMonth         string     `json:"term_month"`
	DistributionType  string     `json:"distribution_type"`
	TotalAmountCents  int64      `json:"total_amount_cents"`
	IsReversed        bool       `json:"is_reversed"`
	ReversalAccrualID *uuid.UUID `json:"reversal_accrual_id,omitempty"`
	CreatedBy         *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// Debt represents fy-debts
type Debt struct {
	ID                 uuid.UUID  `json:"id"`
	SiteID             uuid.UUID  `json:"site_id"`
	UnitID             uuid.UUID  `json:"unit_id"`
	UnitNumber         string     `json:"unit_number,omitempty"`
	BlockName          string     `json:"block_name,omitempty"`
	UserID             uuid.UUID  `json:"user_id"`
	UserName           string     `json:"user_name,omitempty"`
	AccrualID          *uuid.UUID `json:"accrual_id,omitempty"`
	Title              string     `json:"title"`
	DebtType           string     `json:"debt_type"`
	TermMonth          string     `json:"term_month"`
	PrincipalCents     int64      `json:"principal_cents"`
	PaidPrincipalCents int64      `json:"paid_principal_cents"`
	PaidInterestCents  int64      `json:"paid_interest_cents"`
	RemainingCents     int64      `json:"remaining_cents"`
	CalculatedFeeCents int64      `json:"calculated_fee_cents,omitempty"`
	DueDate            time.Time  `json:"due_date"`
	Status             string     `json:"status"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Payment represents fy-payments
type Payment struct {
	ID               uuid.UUID  `json:"id"`
	SiteID           uuid.UUID  `json:"site_id"`
	UnitID           uuid.UUID  `json:"unit_id"`
	UnitNumber       string     `json:"unit_number,omitempty"`
	UserID           uuid.UUID  `json:"user_id"`
	UserName         string     `json:"user_name,omitempty"`
	PaymentDate      time.Time  `json:"payment_date"`
	AmountCents      int64      `json:"amount_cents"`
	UnallocatedCents int64      `json:"unallocated_cents"`
	PaymentChannel   string     `json:"payment_channel"`
	BankReference    string     `json:"bank_reference,omitempty"`
	Description      string     `json:"description,omitempty"`
	IdempotencyKey   string     `json:"idempotency_key,omitempty"`
	CreatedBy        *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// Expense represents fy-expenses
type Expense struct {
	ID            uuid.UUID  `json:"id"`
	SiteID        uuid.UUID  `json:"site_id"`
	PeriodID      *uuid.UUID `json:"period_id,omitempty"`
	Category      string     `json:"category"`
	Title         string     `json:"title"`
	VendorName    string     `json:"vendor_name"`
	InvoiceNumber string     `json:"invoice_number,omitempty"`
	InvoiceDate   time.Time  `json:"invoice_date"`
	DueDate       *time.Time `json:"due_date,omitempty"`
	AmountCents   int64      `json:"amount_cents"`
	PaidCents     int64      `json:"paid_cents"`
	Status        string     `json:"status"`
	DocumentURL   string     `json:"document_url,omitempty"`
	CreatedBy     *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// LedgerAccount represents fy-ledger-accounts
type LedgerAccount struct {
	ID           uuid.UUID `json:"id"`
	SiteID       uuid.UUID `json:"site_id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	AccountType  string    `json:"account_type"`
	BalanceCents int64     `json:"balance_cents"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

// JournalEntry represents fy-journal-entries
type JournalEntry struct {
	ID               uuid.UUID     `json:"id"`
	SiteID           uuid.UUID     `json:"site_id"`
	PeriodID         uuid.UUID     `json:"period_id"`
	EntryNumber      int64         `json:"entry_number"`
	EntryDate        time.Time     `json:"entry_date"`
	Description      string        `json:"description"`
	IsReversal       bool          `json:"is_reversal"`
	ReversedEntryID  *uuid.UUID    `json:"reversed_entry_id,omitempty"`
	Lines            []JournalLine `json:"lines,omitempty"`
	TotalDebitCents  int64         `json:"total_debit_cents"`
	TotalCreditCents int64         `json:"total_credit_cents"`
	CreatedAt        time.Time     `json:"created_at"`
}

// JournalLine represents fy-journal-lines
type JournalLine struct {
	ID             uuid.UUID `json:"id"`
	JournalEntryID uuid.UUID `json:"journal_entry_id"`
	AccountID      uuid.UUID `json:"account_id"`
	AccountCode    string    `json:"account_code,omitempty"`
	AccountName    string    `json:"account_name,omitempty"`
	DebitCents     int64     `json:"debit_cents"`
	CreditCents    int64     `json:"credit_cents"`
	Description    string    `json:"description,omitempty"`
}

// BankStatement represents fy-bank-statements
type BankStatement struct {
	ID              uuid.UUID  `json:"id"`
	SiteID          uuid.UUID  `json:"site_id"`
	StatementDate   time.Time  `json:"statement_date"`
	ReferenceNumber string     `json:"reference_number,omitempty"`
	RawDescription  string     `json:"raw_description"`
	AmountCents     int64      `json:"amount_cents"`
	MatchedUnitID   *uuid.UUID `json:"matched_unit_id,omitempty"`
	MatchedUnitNo   string     `json:"matched_unit_no,omitempty"`
	MatchConfidence float64    `json:"match_confidence,omitempty"`
	MatchStatus     string     `json:"match_status"`
	PaymentID       *uuid.UUID `json:"payment_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ClearanceCertificate represents fy-clearance-certificates
type ClearanceCertificate struct {
	ID                      uuid.UUID  `json:"id"`
	SiteID                  uuid.UUID  `json:"site_id"`
	UnitID                  uuid.UUID  `json:"unit_id"`
	UnitNumber              string     `json:"unit_number"`
	BlockName               string     `json:"block_name"`
	UserID                  uuid.UUID  `json:"user_id"`
	UserName                string     `json:"user_name"`
	CertificateNumber       string     `json:"certificate_number"`
	VerificationCode        string     `json:"verification_code"`
	Sha256Hash              string     `json:"sha256_hash"`
	IssuedDate              time.Time  `json:"issued_date"`
	ValidUntil              time.Time  `json:"valid_until"`
	DebtBalanceAtIssueCents int64      `json:"debt_balance_at_issue_cents"`
	IssuedBy                *uuid.UUID `json:"issued_by,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
}

// Announcement represents fy-announcements
type Announcement struct {
	ID          uuid.UUID  `json:"id"`
	SiteID      uuid.UUID  `json:"site_id"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	Priority    string     `json:"priority"`
	IsPublished bool       `json:"is_published"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Decision represents fy-decisions
type Decision struct {
	ID             uuid.UUID  `json:"id"`
	SiteID         uuid.UUID  `json:"site_id"`
	DecisionNumber string     `json:"decision_number"`
	DecisionDate   time.Time  `json:"decision_date"`
	Title          string     `json:"title"`
	Content        string     `json:"content"`
	VoteSummary    string     `json:"vote_summary,omitempty"`
	CreatedBy      *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// MaintenanceRequest represents fy-maintenance-requests
type MaintenanceRequest struct {
	ID              uuid.UUID  `json:"id"`
	SiteID          uuid.UUID  `json:"site_id"`
	UnitID          *uuid.UUID `json:"unit_id,omitempty"`
	UnitNumber      string     `json:"unit_number,omitempty"`
	UserID          uuid.UUID  `json:"user_id"`
	UserName        string     `json:"user_name,omitempty"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Category        string     `json:"category"`
	Status          string     `json:"status"`
	Priority        string     `json:"priority"`
	ResolutionNotes string     `json:"resolution_notes,omitempty"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// AuditLog represents fy-audit-logs
type AuditLog struct {
	ID          uuid.UUID   `json:"id"`
	SiteID      *uuid.UUID  `json:"site_id,omitempty"`
	UserID      *uuid.UUID  `json:"user_id,omitempty"`
	Action      string      `json:"action"`
	EntityTable string      `json:"entity_table"`
	EntityID    uuid.UUID   `json:"entity_id"`
	OldValues   interface{} `json:"old_values,omitempty"`
	NewValues   interface{} `json:"new_values,omitempty"`
	IPAddress   string      `json:"ip_address,omitempty"`
	UserAgent   string      `json:"user_agent,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

// API Error standard
type APIError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
