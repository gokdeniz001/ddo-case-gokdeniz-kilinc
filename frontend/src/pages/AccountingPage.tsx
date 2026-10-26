import React, { useEffect, useState } from 'react';
import { BookOpen, CheckCircle2, AlertCircle } from 'lucide-react';
import { api, formatCents } from '../services/api';

interface AccountingPageProps {
  onNotify: (msg: { type: 'success' | 'error'; text: string }) => void;
}

export const AccountingPage: React.FC<AccountingPageProps> = ({ onNotify }) => {
  const [accounts, setAccounts] = useState<any[]>([]);
  const [entries, setEntries] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  const loadData = async () => {
    setLoading(true);
    try {
      const [accList, entryList] = await Promise.all([
        api.getLedgerAccounts().catch(() => []),
        api.getJournalEntries().catch(() => []),
      ]);
      setAccounts(accList);
      setEntries(entryList);
    } catch (err: any) {
      onNotify({ type: 'error', text: err.message || 'Muhasebe verileri yüklenemedi' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  return (
    <div className="page-container">
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: '1.75rem', marginBottom: 6 }}>Çift Taraflı Muhasebe & Defter-i Kebir (Task 4)</h1>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.95rem' }}>
          Tek Düzen Hesap Planı uyumlu çift taraflı kayıt (Double-Entry Bookkeeping), mizan ve yevmiye defteri.
        </p>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24, marginBottom: 24 }}>
        {/* Chart of Accounts */}
        <div className="glass-card" style={{ padding: 24 }}>
          <h3 style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
            <BookOpen size={20} color="var(--primary)" /> Hesap Planı ve Mizan (Özet)
          </h3>

          <div className="table-responsive">
            <table className="data-table">
              <thead>
                <tr>
                  <th>KOD</th>
                  <th>HESAP ADI</th>
                  <th>TÜR</th>
                  <th>BAKİYE</th>
                </tr>
              </thead>
              <tbody>
                {accounts.map((a) => (
                  <tr key={a.id}>
                    <td>
                      <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 600, color: 'var(--primary)' }}>
                        {a.code}
                      </span>
                    </td>
                    <td>{a.name}</td>
                    <td>
                      <span className="role-badge" style={{ background: 'rgba(255,255,255,0.05)' }}>
                        {a.account_type}
                      </span>
                    </td>
                    <td style={{ fontWeight: 600 }}>{formatCents(a.balance_cents || 0)}</td>
                  </tr>
                ))}
                {accounts.length === 0 && (
                  <tr>
                    <td colSpan={4} style={{ textAlign: 'center', padding: 16, color: 'var(--text-dim)' }}>
                      Hesap planı yüklenemedi.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>

        {/* Double Entry Rules Card */}
        <div className="glass-card" style={{ padding: 24 }}>
          <h3 style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
            <CheckCircle2 size={20} color="var(--success)" /> Çift Taraflı Denkliği Kuralları
          </h3>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{ padding: '12px 16px', background: 'rgba(255,255,255,0.03)', borderRadius: 8 }}>
              <strong style={{ color: '#fff' }}>Borç (Debit) = Alacak (Credit) Eşitliği</strong>
              <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginTop: 4 }}>
                Sistemde tek taraflı kayıt atılamaz. Tüm tahakkuk, tahsilat ve gider hareketleri en az iki hesap arasında denk olarak kaydedilir.
              </p>
            </div>
            <div style={{ padding: '12px 16px', background: 'rgba(255,255,255,0.03)', borderRadius: 8 }}>
              <strong style={{ color: '#fff' }}>Mali Dönem Kapanışı & Kilit Mekanizması</strong>
              <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginTop: 4 }}>
                Denetçi ve Genel Kurul onayından geçen dönemler 'LOCKED' statüsüne alınarak geriye dönük manipülasyon engellenir.
              </p>
            </div>
            <div style={{ padding: '12px 16px', background: 'rgba(255,255,255,0.03)', borderRadius: 8 }}>
              <strong style={{ color: '#fff' }}>Hata Düzeltme: Yalnızca Ters Kayıt (Reversal)</strong>
              <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginTop: 4 }}>
                Muhasebe defterinde satır silme işlemi yapılamaz; yanlış işlem karşılığı ters fiş ile dengelenir.
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Journal Entries Table */}
      <div className="glass-card" style={{ padding: 24 }}>
        <h3 style={{ marginBottom: 16 }}>Son Yevmiye Fişleri (Defter-i Kebir Hareketleri)</h3>

        <div className="table-responsive">
          <table className="data-table">
            <thead>
              <tr>
                <th>FİŞ NO</th>
                <th>TARİH</th>
                <th>AÇIKLAMA</th>
                <th>BORÇ TOPLAMI</th>
                <th>ALACAK TOPLAMI</th>
                <th>DURUM</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e) => (
                <tr key={e.id}>
                  <td>
                    <span style={{ fontFamily: 'var(--font-mono)' }}>#{e.entry_number}</span>
                  </td>
                  <td style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                    {e.entry_date ? e.entry_date.substring(0, 10) : '—'}
                  </td>
                  <td>{e.description}</td>
                  <td style={{ fontWeight: 600 }}>{formatCents(e.total_debit_cents || 0)}</td>
                  <td style={{ fontWeight: 600 }}>{formatCents(e.total_credit_cents || 0)}</td>
                  <td>
                    <span className={`status-pill ${e.is_reversal ? 'unpaid' : 'paid'}`}>
                      {e.is_reversal ? 'TERS KAYIT' : 'ONAYLI'}
                    </span>
                  </td>
                </tr>
              ))}
              {entries.length === 0 && (
                <tr>
                  <td colSpan={6} style={{ textAlign: 'center', padding: 24, color: 'var(--text-dim)' }}>
                    Yevmiye kaydı bulunmuyor.
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
