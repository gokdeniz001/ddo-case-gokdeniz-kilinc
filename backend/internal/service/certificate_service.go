package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/siteyonet/siteyonet/internal/domain"
)

type CertificateService struct {
	secretSalt string
}

func NewCertificateService() *CertificateService {
	return &CertificateService{
		secretSalt: "ddo-audit-clearance-hash-salt-0c7cc1",
	}
}

// IssueClearanceCertificate generates an authenticatable, signed clearance certificate
func (s *CertificateService) IssueClearanceCertificate(
	site domain.Site,
	unit domain.Unit,
	user domain.User,
	totalDebtBalanceCents int64,
	validDays int,
	issuedBy uuid.UUID,
) (*domain.ClearanceCertificate, error) {
	if totalDebtBalanceCents > 0 {
		return nil, fmt.Errorf("borcu yoktur belgesi verilemez: Dairenin %s tutarında ödenmemiş borcu bulunmaktadır", domain.FormatCents(totalDebtBalanceCents))
	}

	issuedDate := time.Now().Truncate(24 * time.Hour)
	validUntil := issuedDate.AddDate(0, 0, validDays)
	certID := uuid.New()

	certNumber := fmt.Sprintf("BYB-%s-%s-%s", 
		strings.ToUpper(site.Code), 
		time.Now().Format("20060102"), 
		strings.ToUpper(certID.String()[:8]))

	verificationCode := strings.ToUpper(fmt.Sprintf("%s-%s", 
		uuid.New().String()[:4], 
		uuid.New().String()[9:13]))

	// Compute SHA-256 cryptographic proof
	rawProof := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%s",
		site.ID.String(),
		unit.ID.String(),
		user.ID.String(),
		certNumber,
		verificationCode,
		issuedDate.Format("2006-01-02"),
		totalDebtBalanceCents,
		s.secretSalt,
	)

	hasher := sha256.New()
	hasher.Write([]byte(rawProof))
	shaProof := hex.EncodeToString(hasher.Sum(nil))

	return &domain.ClearanceCertificate{
		ID:                      certID,
		SiteID:                  site.ID,
		UnitID:                  unit.ID,
		UnitNumber:              unit.UnitNumber,
		BlockName:               unit.BlockName,
		UserID:                  user.ID,
		UserName:                user.FullName,
		CertificateNumber:       certNumber,
		VerificationCode:        verificationCode,
		Sha256Hash:              shaProof,
		IssuedDate:              issuedDate,
		ValidUntil:              validUntil,
		DebtBalanceAtIssueCents: totalDebtBalanceCents,
		IssuedBy:                &issuedBy,
		CreatedAt:               time.Now(),
	}, nil
}

// VerifyHash checks if the certificate content matches the cryptographic SHA-256 hash
func (s *CertificateService) VerifyHash(cert domain.ClearanceCertificate) bool {
	rawProof := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%s",
		cert.SiteID.String(),
		cert.UnitID.String(),
		cert.UserID.String(),
		cert.CertificateNumber,
		cert.VerificationCode,
		cert.IssuedDate.Format("2006-01-02"),
		cert.DebtBalanceAtIssueCents,
		s.secretSalt,
	)

	hasher := sha256.New()
	hasher.Write([]byte(rawProof))
	expected := hex.EncodeToString(hasher.Sum(nil))

	return expected == cert.Sha256Hash
}
