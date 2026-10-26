import React, { useEffect, useState } from 'react';
import { Calculator, CheckCircle2, History } from 'lucide-react';
import { api, formatCents } from '../services/api';

interface AccrualsPageProps {
  onNotify: (msg: { type: 'success' | 'error'; text: string }) => void;
}

export const AccrualsPage: React.FC<AccrualsPageProps> = ({ onNotify }) => {
  const [accrualTitle, setAccrualTitle] = useState('2026 Kasım Dönemi Aidatı');
  const [accrualAmountTL, setAccrualAmountTL] = useState(28500);
  const [accrualDistType, setAccrualDistType] = useState('LAND_SHARE');
  const [loading, setLoading] = useState(false);
  const [pastAccruals, setPastAccruals] = useState<any[]>([]);

  const loadPastAccruals = async () => {
    try {
      const res = await api.getAccruals();
      setPastAccruals(res || []);
    } catch {
      // ignore
    }
  };

  useEffect(() => {
    loadPastAccruals();
  }, []);

  const handleCreateAccrual = async () => {
    try {
      setLoading(true);
      const periods = await api.getPeriods();
      if (!periods || periods.length === 0) {
        throw new Error('Aktif bir mali dönem bulunamadı');
      }

      await api.createAccrual({
        period_id: periods[0].id,
        title: accrualTitle,
        term_month: '2026-11',
        distribution_type: accrualDistType,
        total_amount_cents: Math.round(accrualAmountTL * 100),
        due_day: 15,
      });

      onNotify({
        type: 'success',
        text: 'Tahakkuk En Büyük Kalan Yöntemi (Hare-Niemeyer) ile kuruşu kuruşuna paylaştırıldı ve borçlar oluşturuldu!',
      });
      await loadPastAccruals();
    } catch (err: any) {
      onNotify({ type: 'error', text: err.message || 'Tahakkuk oluşturulamadı' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="page-container">
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: '1.75rem', marginBottom: 6 }}>Tahakkuk & Dağıtım Motoru (Task 1)</h1>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.95rem' }}>
          KMK Madde 20 uyumlu En Büyük Kalan Yöntemi (Hare-Niemeyer) ile sıfır kuruş kaybı garantili borçlandırma.
        </p>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24, marginBottom: 24 }}>
        {/* Simulation Form Card */}
        <div className="glass-card" style={{ padding: 24 }}>
          <h3 style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
            <Calculator size={20} color="var(--primary)" /> Canlı Tahakkuk Çalıştır
          </h3>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div>
              <label style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: 6, display: 'block' }}>
                Tahakkuk Başlığı
              </label>
              <input
                className="input-field"
                type="text"
                value={accrualTitle}
                onChange={(e) => setAccrualTitle(e.target.value)}
              />
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
              <div>
                <label style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: 6, display: 'block' }}>
                  Toplam Bütçe (TL)
                </label>
                <input
                  className="input-field"
                  type="number"
                  value={accrualAmountTL}
                  onChange={(e) => setAccrualAmountTL(Number(e.target.value))}
                />
              </div>

              <div>
                <label style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: 6, display: 'block' }}>
                  Dağıtım Yöntemi
                </label>
                <select
                  className="input-field"
                  value={accrualDistType}
                  onChange={(e) => setAccrualDistType(e.target.value)}
                >
                  <option value="LAND_SHARE">Arsa Payı (KMK Md. 20)</option>
                  <option value="EQUAL">Eşit Dağıtım (1/N)</option>
                  <option value="AREA_M2">Brüt Alan (m²)</option>
                  <option value="RESIDENT_COUNT">Kişi Sayısı</option>
                </select>
              </div>
            </div>

            <div
              style={{
                padding: '12px 16px',
                background: 'rgba(59, 130, 246, 0.08)',
                border: '1px solid rgba(59, 130, 246, 0.2)',
                borderRadius: 'var(--radius-sm)',
                fontSize: '0.85rem',
                color: 'var(--text-muted)',
              }}
            >
              ℹ️ <strong>En Büyük Kalan Yöntemi:</strong> Küsürat yuvarlamasından doğan 1 kuruşluk farklar, sıralı olarak en yüksek kesirli kalan dairelere paylaştırılarak toplam dağıtılan tutarın tam bütçeye eşitliği garanti edilir.
            </div>

            <button
              className="btn btn-primary"
              style={{ padding: '12px 20px', marginTop: 8 }}
              onClick={handleCreateAccrual}
              disabled={loading}
            >
              {loading ? 'Dağıtılıyor...' : 'Tahakkuku Çalıştır & Fiş Kes'}
            </button>
          </div>
        </div>

        {/* Algorithm Info Card */}
        <div className="glass-card" style={{ padding: 24 }}>
          <h3 style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
            <CheckCircle2 size={20} color="var(--success)" /> DDO Denetim & KMK Kuralları
          </h3>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 12, fontSize: '0.9rem', color: 'var(--text-muted)' }}>
            <div style={{ padding: '10px 14px', background: 'rgba(255,255,255,0.03)', borderRadius: 8 }}>
              <strong style={{ color: '#fff' }}>1. Kapıcı Dairesi Muafiyeti:</strong>
              <p style={{ marginTop: 4 }}>Kapıcı veya sığınağa ait muaf daireler dağıtıma dahil edilmez, payları diğer dairelere orantılanır.</p>
            </div>
            <div style={{ padding: '10px 14px', background: 'rgba(255,255,255,0.03)', borderRadius: 8 }}>
              <strong style={{ color: '#fff' }}>2. Otomatik Yevmiye Fişi (120 / 600):</strong>
              <p style={{ marginTop: 4 }}>Her tahakkuk işleminde çift taraflı muhasebe fişi (Borç: 120 Aidat Alacakları, Alacak: 600 Aidat Gelirleri) anında kesilir.</p>
            </div>
            <div style={{ padding: '10px 14px', background: 'rgba(255,255,255,0.03)', borderRadius: 8 }}>
              <strong style={{ color: '#fff' }}>3. İptal / Ters Kayıt Koruması:</strong>
              <p style={{ marginTop: 4 }}>Geçmiş tahakkuklar silinemez, ancak gerekçe belirtilerek ters kayıt fişi ile dengelenir.</p>
            </div>
          </div>
        </div>
      </div>

      {/* Past Accruals Table */}
      <div className="glass-card" style={{ padding: 24 }}>
        <h3 style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
          <History size={18} color="var(--primary)" /> Geçmiş Dönem Tahakkuk Kayıtları
        </h3>

        <div className="table-responsive">
          <table className="data-table">
            <thead>
              <tr>
                <th>DÖNEM</th>
                <th>BAŞLIK</th>
                <th>YÖNTEM</th>
                <th>TOPLAM TUTAR</th>
                <th>DURUM</th>
                <th>KAYIT TARİHİ</th>
              </tr>
            </thead>
            <tbody>
              {pastAccruals.map((a) => (
                <tr key={a.id}>
                  <td><strong style={{ fontFamily: 'var(--font-mono)' }}>{a.term_month}</strong></td>
                  <td>{a.title}</td>
                  <td><span className="role-badge" style={{ background: 'rgba(59, 130, 246, 0.15)', color: '#60a5fa' }}>{a.distribution_type}</span></td>
                  <td style={{ fontWeight: 600 }}>{formatCents(a.total_amount_cents)}</td>
                  <td>
                    <span className={`status-pill ${a.is_reversed ? 'unpaid' : 'paid'}`}>
                      {a.is_reversed ? 'İPTAL/TERS KAYIT' : 'AKTİF'}
                    </span>
                  </td>
                  <td style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                    {a.created_at ? a.created_at.substring(0, 10) : '—'}
                  </td>
                </tr>
              ))}
              {pastAccruals.length === 0 && (
                <tr>
                  <td colSpan={6} style={{ textAlign: 'center', padding: 20, color: 'var(--text-dim)' }}>
                    Kayıtlı tahakkuk bulunmuyor.
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
