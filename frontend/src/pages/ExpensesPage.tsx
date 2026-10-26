import React, { useEffect, useState } from 'react';
import { Receipt, Plus, FileText, CheckCircle2 } from 'lucide-react';
import { api, Expense, formatCents } from '../services/api';

interface ExpensesPageProps {
  onNotify: (msg: { type: 'success' | 'error'; text: string }) => void;
}

export const ExpensesPage: React.FC<ExpensesPageProps> = ({ onNotify }) => {
  const [expenses, setExpenses] = useState<Expense[]>([]);
  const [loading, setLoading] = useState(true);
  const [categoryFilter, setCategoryFilter] = useState('ALL');

  // Form State
  const [showAddModal, setShowAddModal] = useState(false);
  const [title, setTitle] = useState('');
  const [category, setCategory] = useState('ELEVATOR');
  const [vendorName, setVendorName] = useState('');
  const [invoiceNumber, setInvoiceNumber] = useState('');
  const [amountTL, setAmountTL] = useState(4500);

  const loadExpenses = async () => {
    setLoading(true);
    try {
      const data = await api.getExpenses();
      setExpenses(data || []);
    } catch (err: any) {
      onNotify({ type: 'error', text: err.message || 'Giderler yüklenemedi' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadExpenses();
  }, []);

  const handleCreateExpense = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title || !vendorName) {
      onNotify({ type: 'error', text: 'Lütfen zorunlu alanları doldurun' });
      return;
    }

    try {
      await api.createExpense({
        category,
        title,
        vendor_name: vendorName,
        invoice_number: invoiceNumber || undefined,
        amount_cents: Math.round(amountTL * 100),
      });

      onNotify({
        type: 'success',
        text: 'Gider faturası kaydedildi ve muhasebe yevmiye kaydı (770 Genel Yönetim Giderleri) kesildi!',
      });
      setShowAddModal(false);
      setTitle('');
      setVendorName('');
      setInvoiceNumber('');
      await loadExpenses();
    } catch (err: any) {
      onNotify({ type: 'error', text: err.message || 'Gider kaydedilemedi' });
    }
  };

  const filteredExpenses = expenses.filter(
    (e) => categoryFilter === 'ALL' || e.category === categoryFilter
  );

  const totalExpenseCents = filteredExpenses.reduce((sum, e) => sum + e.amount_cents, 0);

  return (
    <div className="page-container">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <div>
          <h1 style={{ fontSize: '1.75rem', marginBottom: 6 }}>Gider & Fatura Yönetimi (Feature 4)</h1>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.95rem' }}>
            Ortak alan bakım, enerji, güvenlik ve tedarikçi faturalarının çift taraflı muhasebe entegrasyonu.
          </p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowAddModal(true)}>
          <Plus size={18} /> Yeni Gider Faturası Ekle
        </button>
      </div>

      {/* Filter & Summary Card */}
      <div
        className="glass-card"
        style={{
          padding: '16px 20px',
          marginBottom: 20,
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          flexWrap: 'wrap',
          gap: 16,
        }}
      >
        <div className="filter-pills-row">
          {[
            { id: 'ALL', label: '✨ Tüm Kategoriler' },
            { id: 'ELEVATOR', label: '🛗 Asansör' },
            { id: 'ENERGY', label: '⚡ Elektrik' },
            { id: 'SECURITY', label: '🛡️ Güvenlik' },
            { id: 'CLEANING', label: '🧹 Temizlik' },
            { id: 'LANDSCAPING', label: '🌳 Peyzaj' },
          ].map((cat) => (
            <button
              key={cat.id}
              type="button"
              className={`filter-pill-btn ${categoryFilter === cat.id ? 'active' : ''}`}
              onClick={() => setCategoryFilter(cat.id)}
            >
              {cat.label}
            </button>
          ))}
        </div>

        <div>
          <span style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginRight: 8 }}>
            Kategori Toplamı:
          </span>
          <strong style={{ fontSize: '1.2rem', color: 'var(--danger)' }}>
            {formatCents(totalExpenseCents)}
          </strong>
        </div>
      </div>

      {/* Add Modal */}
      {showAddModal && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.6)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
          }}
        >
          <div className="glass-card" style={{ width: 480, padding: 24, background: '#111827' }}>
            <h3 style={{ marginBottom: 16 }}>Yeni Gider Faturası Kaydet</h3>
            <form onSubmit={handleCreateExpense} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div>
                <label style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: 4, display: 'block' }}>
                  Gider Başlığı *
                </label>
                <input
                  className="input-field"
                  placeholder="Örn: Blok Merdiven Boşluğu Boya Bakımı"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  required
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
                <div>
                  <label style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: 4, display: 'block' }}>
                    Kategori
                  </label>
                  <select className="input-field" value={category} onChange={(e) => setCategory(e.target.value)}>
                    <option value="ELEVATOR">Asansör</option>
                    <option value="ENERGY">Elektrik & Enerji</option>
                    <option value="SECURITY">Güvenlik</option>
                    <option value="CLEANING">Temizlik</option>
                    <option value="LANDSCAPING">Peyzaj</option>
                    <option value="REPAIR">Onarım</option>
                  </select>
                </div>
                <div>
                  <label style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: 4, display: 'block' }}>
                    Tutar (TL) *
                  </label>
                  <input
                    className="input-field"
                    type="number"
                    value={amountTL}
                    onChange={(e) => setAmountTL(Number(e.target.value))}
                    required
                  />
                </div>
              </div>

              <div>
                <label style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: 4, display: 'block' }}>
                  Tedarikçi / Firma Adı *
                </label>
                <input
                  className="input-field"
                  placeholder="Örn: KONE Asansör A.Ş."
                  value={vendorName}
                  onChange={(e) => setVendorName(e.target.value)}
                  required
                />
              </div>

              <div>
                <label style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: 4, display: 'block' }}>
                  Fatura Numarası
                </label>
                <input
                  className="input-field"
                  placeholder="Örn: GIB202600000123"
                  value={invoiceNumber}
                  onChange={(e) => setInvoiceNumber(e.target.value)}
                />
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, marginTop: 10 }}>
                <button type="button" className="btn btn-secondary" onClick={() => setShowAddModal(false)}>
                  Vazgeç
                </button>
                <button type="submit" className="btn btn-primary">
                  Kaydet & Yevmiye Fişi Kes
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Expenses Table */}
      <div className="glass-card" style={{ padding: 24 }}>
        <div className="table-responsive">
          <table className="data-table">
            <thead>
              <tr>
                <th>TARİH</th>
                <th>FATURA NO</th>
                <th>GİDER BAŞLIĞI</th>
                <th>KATEGORİ</th>
                <th>TEDARİKÇİ</th>
                <th>TUTAR</th>
                <th>DURUM</th>
              </tr>
            </thead>
            <tbody>
              {filteredExpenses.map((exp) => (
                <tr key={exp.id}>
                  <td style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                    {exp.invoice_date ? exp.invoice_date.substring(0, 10) : '—'}
                  </td>
                  <td>
                    <span style={{ fontFamily: 'var(--font-mono)', fontSize: '0.85rem' }}>
                      {exp.invoice_number || '—'}
                    </span>
                  </td>
                  <td>
                    <strong style={{ color: '#fff' }}>{exp.title}</strong>
                  </td>
                  <td>
                    <span className="role-badge" style={{ background: 'rgba(239, 68, 68, 0.15)', color: '#f87171' }}>
                      {exp.category}
                    </span>
                  </td>
                  <td>{exp.vendor_name}</td>
                  <td style={{ fontWeight: 600, color: 'var(--danger)' }}>
                    {formatCents(exp.amount_cents)}
                  </td>
                  <td>
                    <span className="status-pill paid">{exp.status}</span>
                  </td>
                </tr>
              ))}
              {filteredExpenses.length === 0 && !loading && (
                <tr>
                  <td colSpan={7} style={{ textAlign: 'center', padding: 24, color: 'var(--text-dim)' }}>
                    Kayıtlı gider faturası bulunmuyor.
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
