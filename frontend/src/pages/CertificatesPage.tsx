import React, { useEffect, useState } from 'react';
import { Award, ShieldCheck, Search, CheckCircle2, AlertCircle } from 'lucide-react';
import { api, ClearanceCertificate, formatCents } from '../services/api';

interface CertificatesPageProps {
  onNotify: (msg: { type: 'success' | 'error'; text: string }) => void;
}

export const CertificatesPage: React.FC<CertificatesPageProps> = ({ onNotify }) => {
  const [certificates, setCertificates] = useState<ClearanceCertificate[]>([]);
  const [units, setUnits] = useState<any[]>([]);
  const [selectedUnitId, setSelectedUnitId] = useState('');
  const [certBlockFilter, setCertBlockFilter] = useState('ALL');
  const [generating, setGenerating] = useState(false);

  const selectedUnit = units.find((u) => u.id === selectedUnitId);

  // Verification state
  const [verifyCode, setVerifyCode] = useState('');
  const [verificationResult, setVerificationResult] = useState<any>(null);
  const [verifying, setVerifying] = useState(false);

  const loadData = async () => {
    try {
      const [certs, unitList] = await Promise.all([
        api.getCertificates(),
        api.getUnits(),
      ]);
      setCertificates(certs || []);
      setUnits(unitList || []);
      if (unitList && unitList.length > 0) {
        setSelectedUnitId(unitList[0].id);
      }
    } catch {
      // ignore
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleIssueCertificate = async () => {
    if (!selectedUnitId) return;
    setGenerating(true);
    try {
      const cert = await api.issueCertificate(selectedUnitId);
      onNotify({
        type: 'success',
        text: `Borcu Yoktur Belgesi başarıyla oluşturuldu! Kod: ${cert.verification_code}`,
      });
      await loadData();
    } catch (err: any) {
      onNotify({
        type: 'error',
        text: err.message || 'Borcu Yoktur Belgesi verilemedi (Aktif borç bakiyesi mevcut olabilir)',
      });
    } finally {
      setGenerating(false);
    }
  };

  const handleVerify = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!verifyCode.trim()) return;
    setVerifying(true);
    setVerificationResult(null);
    try {
      const res = await api.verifyCertificate(verifyCode.trim());
      setVerificationResult(res);
      onNotify({ type: 'success', text: 'Belge doğrulandı: Geçerli ve orijinal!' });
    } catch (err: any) {
      setVerificationResult({ isValid: false, message: err.message || 'Geçersiz veya sahte doğrulama kodu' });
      onNotify({ type: 'error', text: 'Doğrulama başarısız: Belge bulunamadı veya değiştirilmiş' });
    } finally {
      setVerifying(false);
    }
  };

  return (
    <div className="page-container">
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: '1.75rem', marginBottom: 6 }}>Borcu Yoktur Belgesi (Task 4)</h1>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.95rem' }}>
          Tapu ve noter devirlerinde geçerli, SHA-256 dijital imzalı ve QR kodlu resmi temize çıkarma belgesi.
        </p>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24, marginBottom: 24 }}>
        {/* Issue Certificate Card */}
        <div className="glass-card" style={{ padding: 24 }}>
          <h3 style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
            <Award size={20} color="var(--primary)" /> Yeni Belge Düzenle (Tapu / Noter)
          </h3>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                <label style={{ fontSize: '0.85rem', color: 'var(--text-muted)', fontWeight: 600 }}>
                  Temize Çıkarılacak Daireyi Kutuya Tıklayarak Seçin:
                </label>
                {selectedUnit && (
                  <span style={{ fontSize: '0.8rem', color: 'var(--primary)', fontWeight: 600 }}>
                    Seçili: {selectedUnit.block_name} No: {selectedUnit.unit_number}
                  </span>
                )}
              </div>

              {/* Block Filter Pills */}
              <div className="filter-pills-row" style={{ marginBottom: 12 }}>
                <button
                  type="button"
                  className={`filter-pill-btn ${certBlockFilter === 'ALL' ? 'active' : ''}`}
                  onClick={() => setCertBlockFilter('ALL')}
                >
                  Tümü ({units.length})
                </button>
                <button
                  type="button"
                  className={`filter-pill-btn ${certBlockFilter === 'A Blok' ? 'active' : ''}`}
                  onClick={() => setCertBlockFilter('A Blok')}
                >
                  A Blok
                </button>
                <button
                  type="button"
                  className={`filter-pill-btn ${certBlockFilter === 'B Blok' ? 'active' : ''}`}
                  onClick={() => setCertBlockFilter('B Blok')}
                >
                  B Blok
                </button>
              </div>

              {/* Box Grid */}
              <div
                className="box-grid"
                style={{
                  maxHeight: '190px',
                  overflowY: 'auto',
                  padding: '4px',
                  border: '1px solid var(--border-glass)',
                  borderRadius: 'var(--radius-sm)',
                  background: 'rgba(0,0,0,0.2)',
                }}
              >
                {units
                  .filter((u) => certBlockFilter === 'ALL' || u.block_name === certBlockFilter)
                  .map((u) => {
                    const isSelected = selectedUnitId === u.id;
                    return (
                      <div
                        key={u.id}
                        className={`select-box-card ${isSelected ? 'active' : ''}`}
                        onClick={() => setSelectedUnitId(u.id)}
                      >
                        <div className="select-box-title">
                          <span>{u.block_name[0]}-{u.unit_number}</span>
                          {isSelected && <CheckCircle2 size={14} color="var(--primary)" />}
                        </div>
                        <div className="select-box-desc" title={u.owner_name || 'Mülk Sahibi'}>
                          {u.owner_name || 'Mülk Sahibi'}
                        </div>
                      </div>
                    );
                  })}
              </div>
            </div>

            <div
              style={{
                padding: '12px 16px',
                background: 'rgba(234, 179, 8, 0.08)',
                border: '1px solid rgba(234, 179, 8, 0.25)',
                borderRadius: 'var(--radius-sm)',
                fontSize: '0.85rem',
                color: 'var(--text-muted)',
              }}
            >
              ⚠️ <strong>Güvenlik Kuralı:</strong> Dairenin geçmiş veya vadesi gelmiş 1 kuruş dahi ödenmemiş borcu varsa sistem belge üretimini derhal reddeder.
            </div>

            <button
              className="btn btn-primary"
              style={{ padding: '12px 20px' }}
              onClick={handleIssueCertificate}
              disabled={generating}
            >
              {generating ? 'Kontrol Ediliyor...' : 'Borç Kontrolü Yap & Belge Üret'}
            </button>
          </div>
        </div>

        {/* Verification Card */}
        <div className="glass-card" style={{ padding: 24 }}>
          <h3 style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
            <ShieldCheck size={20} color="var(--success)" /> Kamu & Noter Belge Doğrulama
          </h3>

          <form onSubmit={handleVerify} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div>
              <label style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: 6, display: 'block' }}>
                Belge Doğrulama Kodu (veya QR Kod Değeri)
              </label>
              <input
                className="input-field"
                placeholder="Örn: CERT-2026-STE01-XXXX"
                value={verifyCode}
                onChange={(e) => setVerifyCode(e.target.value)}
              />
            </div>

            <button type="submit" className="btn btn-secondary" disabled={verifying}>
              <Search size={16} /> {verifying ? 'Sorgulanıyor...' : 'Belgeyi Sorgula'}
            </button>

            {verificationResult && (
              <div
                style={{
                  padding: 16,
                  borderRadius: 'var(--radius-sm)',
                  background: verificationResult.is_valid !== false ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)',
                  border: `1px solid ${verificationResult.is_valid !== false ? 'var(--success)' : 'var(--danger)'}`,
                }}
              >
                {verificationResult.is_valid !== false ? (
                  <div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, color: 'var(--success)', marginBottom: 6 }}>
                      <CheckCircle2 size={18} />
                      <strong>BELGE GEÇERLİ VE ONAYLI</strong>
                    </div>
                    <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                      Daire: <strong>{verificationResult.unit_number}</strong> | Düzenlenme: {verificationResult.issued_date?.substring(0, 10)}
                    </div>
                    <div style={{ fontSize: '0.75rem', fontFamily: 'var(--font-mono)', color: 'var(--text-dim)', marginTop: 4, wordBreak: 'break-all' }}>
                      SHA256: {verificationResult.sha256_hash}
                    </div>
                  </div>
                ) : (
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, color: 'var(--danger)' }}>
                    <AlertCircle size={18} />
                    <span>{verificationResult.message || 'Belge geçersiz'}</span>
                  </div>
                )}
              </div>
            )}
          </form>
        </div>
      </div>

      {/* Issued Certificates List */}
      <div className="glass-card" style={{ padding: 24 }}>
        <h3 style={{ marginBottom: 16 }}>Düzenlenmiş Resmi Temize Çıkarma Belgeleri</h3>

        <div className="table-responsive">
          <table className="data-table">
            <thead>
              <tr>
                <th>BELGE NO</th>
                <th>DAİRE</th>
                <th>MALİK</th>
                <th>DÜZENLEME TARİHİ</th>
                <th>GEÇERLİLİK</th>
                <th>DOĞRULAMA KODU</th>
                <th>DURUM</th>
              </tr>
            </thead>
            <tbody>
              {certificates.map((c) => (
                <tr key={c.id}>
                  <td>
                    <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 600 }}>{c.certificate_number}</span>
                  </td>
                  <td>{c.block_name} No: {c.unit_number}</td>
                  <td>{c.user_name}</td>
                  <td style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                    {c.issued_date ? c.issued_date.substring(0, 10) : '—'}
                  </td>
                  <td style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                    {c.valid_until ? c.valid_until.substring(0, 10) : '—'}
                  </td>
                  <td>
                    <span
                      onClick={() => setVerifyCode(c.verification_code)}
                      title="Kodu kopyala ve sorgula"
                      style={{
                        fontFamily: 'var(--font-mono)',
                        fontSize: '0.85rem',
                        padding: '2px 8px',
                        background: 'rgba(59, 130, 246, 0.1)',
                        border: '1px solid rgba(59, 130, 246, 0.2)',
                        borderRadius: 4,
                        color: '#60a5fa',
                        cursor: 'pointer',
                      }}
                    >
                      {c.verification_code}
                    </span>
                  </td>
                  <td>
                    <span className="status-pill paid">Geçerli</span>
                  </td>
                </tr>
              ))}
              {certificates.length === 0 && (
                <tr>
                  <td colSpan={7} style={{ textAlign: 'center', padding: 20, color: 'var(--text-dim)' }}>
                    Henüz düzenlenmiş belge bulunmuyor.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
