import React, { useEffect, useState } from 'react';
import { StatCard } from '../components/StatCard';
import { api, FinancialSummary, Debt, formatCents } from '../services/api';

interface DashboardPageProps {
  onNotify?: (msg: { type: 'success' | 'error'; text: string }) => void;
}

export const DashboardPage: React.FC<DashboardPageProps> = ({ onNotify }) => {
  const [summary, setSummary] = useState<FinancialSummary | null>(null);
  const [debts, setDebts] = useState<Debt[]>([]);
  const [loading, setLoading] = useState(true);
  const [debtFilter, setDebtFilter] = useState<'ALL' | 'UNPAID' | 'A Blok' | 'B Blok'>('ALL');

  const loadData = async () => {
    setLoading(true);
    try {
      const [sumData, debtData] = await Promise.all([
        api.getFinancialSummary().catch(() => null),
        api.getDebts().catch(() => []),
      ]);
      setSummary(sumData);
      setDebts(debtData);
    } catch (err: any) {
      if (onNotify) onNotify({ type: 'error', text: err.message || 'Veriler yüklenemedi' });
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
        <h1 style={{ fontSize: '1.75rem', marginBottom: 6 }}>Finansal Yönetim Paneli</h1>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.95rem' }}>
          Site aidatları, tahsilat performansı ve nakit akışının kuruş hassasiyetinde canlı takibi.
        </p>
      </div>

      {/* 4 Stat Cards */}
      <div className="stats-grid">
        <StatCard
          label="TOPLAM TAHAKKUK"
          value={formatCents(summary?.total_accruals_cents || 0)}
          meta="3 Dönemlik Resmi Tahakkuk"
          valueColor="#60a5fa"
          metaColor="#93c5fd"
        />
        <StatCard
          label="TOPLAM TAHSİLAT"
          value={formatCents(summary?.total_collected_cents || 0)}
          meta={`Tahsilat Oranı: %${(summary?.collection_rate_percent || 0).toFixed(1)}`}
          valueColor="var(--success)"
          metaColor="var(--success)"
        />
        <StatCard
          label="TOPLAM GİDERLER"
          value={formatCents(summary?.total_expenses_cents || 0)}
          meta="Ortak Alan & Bakım Faturaları"
          valueColor="var(--danger)"
          metaColor="#f87171"
        />
        <StatCard
          label="KALAN BORÇ BAKİYESİ"
          value={formatCents(summary?.total_outstanding_debt_cents || 0)}
          meta="Vadesi Geçmiş / Bekleyen Borçlar"
          valueColor="var(--warning)"
          metaColor="#fbbf24"
        />
      </div>

      {/* Net Cash Flow Banner */}
      <div
        className="glass-card"
        style={{
          padding: '16px 24px',
          marginBottom: 24,
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          borderLeft: '4px solid var(--primary)',
        }}
      >
        <div>
          <span style={{ fontSize: '0.85rem', color: 'var(--text-muted)', textTransform: 'uppercase' }}>
            Kasa / Net Nakit Akışı
          </span>
          <div style={{ fontSize: '1.25rem', fontWeight: 700, color: (summary?.net_cash_flow_cents || 0) >= 0 ? 'var(--success)' : 'var(--danger)' }}>
            {formatCents(summary?.net_cash_flow_cents || 0)}
          </div>
        </div>
        <div style={{ fontSize: '0.85rem', color: 'var(--text-dim)', textAlign: 'right' }}>
          Tahsilat - Gider Dengesi<br />
          <strong style={{ color: '#fff' }}>Kuruş Hassasiyeti Garantili</strong>
        </div>
      </div>

      {/* Recent Debts Table */}
      <div className="glass-card" style={{ padding: 24 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 18, flexWrap: 'wrap', gap: 12 }}>
          <div>
            <h3>Son Daire Borç Kalemleri</h3>
            <span style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
              Top 10 Borç Kalemi {loading && '(Yükleniyor...)'}
            </span>
          </div>

          {/* Quick Filter Boxes */}
          <div className="filter-pills-row">
            <button
              type="button"
              className={`filter-pill-btn ${debtFilter === 'ALL' ? 'active' : ''}`}
              onClick={() => setDebtFilter('ALL')}
            >
              Tümü
            </button>
            <button
              type="button"
              className={`filter-pill-btn ${debtFilter === 'UNPAID' ? 'active' : ''}`}
              onClick={() => setDebtFilter('UNPAID')}
            >
              🔴 Bekleyenler
            </button>
            <button
              type="button"
              className={`filter-pill-btn ${debtFilter === 'A Blok' ? 'active' : ''}`}
              onClick={() => setDebtFilter('A Blok')}
            >
              🏢 A Blok
            </button>
            <button
              type="button"
              className={`filter-pill-btn ${debtFilter === 'B Blok' ? 'active' : ''}`}
              onClick={() => setDebtFilter('B Blok')}
            >
              🏢 B Blok
            </button>
          </div>
        </div>

        <div className="table-responsive">
          <table className="data-table">
            <thead>
              <tr>
                <th>DAİRE</th>
                <th>MALİK / SAKİN</th>
                <th>DÖNEM</th>
                <th>BAŞLIK</th>
                <th>TUTAR</th>
                <th>KALAN</th>
                <th>VADE</th>
                <th>DURUM</th>
              </tr>
            </thead>
            <tbody>
              {debts
                .filter((d) => {
                  if (debtFilter === 'UNPAID') return d.remaining_cents > 0;
                  if (debtFilter === 'A Blok' || debtFilter === 'B Blok') return d.block_name === debtFilter;
                  return true;
                })
                .slice(0, 10)
                .map((d) => (
                <tr key={d.id}>
                  <td style={{ fontWeight: 600 }}>
                    {d.block_name} No: {d.unit_number}
                  </td>
                  <td>{d.user_name || '—'}</td>
                  <td>
                    <span style={{ fontFamily: 'var(--font-mono)', fontSize: '0.85rem' }}>{d.term_month}</span>
                  </td>
                  <td>{d.title}</td>
                  <td style={{ fontWeight: 600 }}>{formatCents(d.principal_cents)}</td>
                  <td style={{ fontWeight: 600, color: d.remaining_cents > 0 ? 'var(--warning)' : 'var(--success)' }}>
                    {formatCents(d.remaining_cents)}
                  </td>
                  <td style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                    {d.due_date ? d.due_date.substring(0, 10) : '—'}
                  </td>
                  <td>
                    <span className={`status-pill ${d.status.toLowerCase()}`}>{d.status}</span>
                  </td>
                </tr>
              ))}
              {debts.length === 0 && !loading && (
                <tr>
                  <td colSpan={8} style={{ textAlign: 'center', padding: 24, color: 'var(--text-dim)' }}>
                    Kayıtlı borç kalemi bulunmuyor.
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
