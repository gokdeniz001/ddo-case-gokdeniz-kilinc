import React, { useEffect, useState } from 'react';
import { Building2, Search, Users, Home, CheckCircle, RotateCcw } from 'lucide-react';
import { api, Unit, normalizeTurkish } from '../services/api';

interface UnitsPageProps {
  onNotify?: (msg: { type: 'success' | 'error'; text: string }) => void;
}

export const UnitsPage: React.FC<UnitsPageProps> = ({ onNotify }) => {
  const [units, setUnits] = useState<Unit[]>([]);
  const [searchTerm, setSearchTerm] = useState('');
  const [blockFilter, setBlockFilter] = useState('ALL');
  const [selectedUnitId, setSelectedUnitId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const loadUnits = async () => {
    setLoading(true);
    try {
      const data = await api.getUnits();
      setUnits(data || []);
    } catch (err: any) {
      if (onNotify) onNotify({ type: 'error', text: err.message || 'Daireler yüklenemedi' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadUnits();
  }, []);

  const normSearch = normalizeTurkish(searchTerm);

  const filteredUnits = units.filter((u) => {
    if (selectedUnitId && u.id !== selectedUnitId) {
      return false;
    }
    const matchBlock = blockFilter === 'ALL' || u.block_name === blockFilter;
    if (!matchBlock) return false;

    if (!normSearch) return true;
    const matchUnitNo = normalizeTurkish(u.unit_number).includes(normSearch);
    const matchOwner = u.owner_name ? normalizeTurkish(u.owner_name).includes(normSearch) : false;
    const matchTenant = u.tenant_name ? normalizeTurkish(u.tenant_name).includes(normSearch) : false;
    const matchBlockName = normalizeTurkish(u.block_name).includes(normSearch);
    return matchUnitNo || matchOwner || matchTenant || matchBlockName;
  });

  const availableUnitsForBoxes = units.filter(
    (u) => blockFilter === 'ALL' || u.block_name === blockFilter
  );

  return (
    <div className="page-container">
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: '1.75rem', marginBottom: 6 }}>Daireler & Sakinler (Feature 1)</h1>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.95rem' }}>
          Bağımsız bölümler, mülk sahipleri, kiracı bilgileri ve arsa payı dağılımı.
        </p>
      </div>

      {/* Box Selection & Filter Bar */}
      <div className="glass-card" style={{ padding: '20px', marginBottom: 20 }}>
        {/* 1. Block Filter Boxes */}
        <div style={{ marginBottom: 16 }}>
          <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', fontWeight: 600, marginBottom: 8 }}>
            BLOK SEÇİMİ (KUTU İLE FİLTRELE):
          </div>
          <div className="filter-pills-row">
            <button
              type="button"
              className={`filter-pill-btn ${blockFilter === 'ALL' ? 'active' : ''}`}
              onClick={() => {
                setBlockFilter('ALL');
                setSelectedUnitId(null);
              }}
            >
              🏢 Tüm Bloklar ({units.length})
            </button>
            <button
              type="button"
              className={`filter-pill-btn ${blockFilter === 'A Blok' ? 'active' : ''}`}
              onClick={() => {
                setBlockFilter('A Blok');
                setSelectedUnitId(null);
              }}
            >
              🏢 A Blok ({units.filter((u) => u.block_name === 'A Blok').length})
            </button>
            <button
              type="button"
              className={`filter-pill-btn ${blockFilter === 'B Blok' ? 'active' : ''}`}
              onClick={() => {
                setBlockFilter('B Blok');
                setSelectedUnitId(null);
              }}
            >
              🏢 B Blok ({units.filter((u) => u.block_name === 'B Blok').length})
            </button>

            {selectedUnitId && (
              <button
                type="button"
                className="filter-pill-btn"
                style={{ borderColor: 'var(--primary)', color: 'var(--primary)' }}
                onClick={() => setSelectedUnitId(null)}
              >
                <RotateCcw size={14} /> Seçimi Kaldır (Tüm Daireleri Göster)
              </button>
            )}
          </div>
        </div>

        {/* 2. Daire Seçim Kutuları (Typo-free Box Grid) */}
        <div style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
            <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', fontWeight: 600 }}>
              DAİRE SEÇİMİ (KUTUYA TIKLAYARAK SEÇİN — YAZIM HATASIZ):
            </span>
            <span style={{ fontSize: '0.78rem', color: 'var(--text-dim)' }}>
              {selectedUnitId ? '1 daire seçili' : 'Tüm daireler listeleniyor'}
            </span>
          </div>

          <div className="box-grid">
            {availableUnitsForBoxes.map((u) => {
              const isSelected = selectedUnitId === u.id;
              return (
                <div
                  key={u.id}
                  className={`select-box-card ${isSelected ? 'active' : ''}`}
                  onClick={() => setSelectedUnitId(isSelected ? null : u.id)}
                >
                  <div className="select-box-title">
                    <span>{u.block_name[0]}-{u.unit_number}</span>
                    {isSelected && <CheckCircle size={14} color="var(--primary)" />}
                  </div>
                  <div className="select-box-desc" title={u.owner_name || 'Mülk Sahibi'}>
                    {u.owner_name || 'Boş Daire'}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* 3. Search Bar with Turkish normalization as optional search */}
        <div style={{ display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap', paddingTop: 10, borderTop: '1px solid var(--border-glass)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flex: 1, minWidth: 240 }}>
            <Search size={18} color="var(--text-muted)" />
            <input
              className="input-field"
              placeholder="İsteğe bağlı isim / no ile filtrele (büyük/küçük harf duyarsız)..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </div>
          <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
            Görüntülenen: <strong>{filteredUnits.length}</strong> / {units.length} daire
          </div>
        </div>
      </div>

      {/* Units Table */}
      <div className="glass-card" style={{ padding: 24 }}>
        <div className="table-responsive">
          <table className="data-table">
            <thead>
              <tr>
                <th>BLOK / NO</th>
                <th>KAT</th>
                <th>MALİK (MÜLK SAHİBİ)</th>
                <th>KİRACI (SAKİN)</th>
                <th>ARSA PAYI</th>
                <th>ALAN (M²)</th>
                <th>KİŞİ SAYISI</th>
                <th>DURUM</th>
              </tr>
            </thead>
            <tbody>
              {filteredUnits.map((u) => (
                <tr key={u.id}>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <Home size={16} color="var(--primary)" />
                      <strong>{u.block_name} No: {u.unit_number}</strong>
                    </div>
                  </td>
                  <td>{u.floor_number}. Kat</td>
                  <td>
                    <strong style={{ color: '#fff' }}>{u.owner_name || '—'}</strong>
                  </td>
                  <td>
                    <span style={{ color: u.tenant_name ? 'var(--text-main)' : 'var(--text-dim)' }}>
                      {u.tenant_name || '— (Malik Oturuyor)'}
                    </span>
                  </td>
                  <td><span style={{ fontFamily: 'var(--font-mono)' }}>{u.land_share} / 10000</span></td>
                  <td>{u.gross_area_m2} m²</td>
                  <td>
                    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                      <Users size={14} color="var(--text-dim)" /> {u.resident_count}
                    </span>
                  </td>
                  <td>
                    {u.is_exempt ? (
                      <span className="role-badge" style={{ background: 'rgba(239, 68, 68, 0.15)', color: '#f87171' }}>
                        Muaf (Kapıcı)
                      </span>
                    ) : (
                      <span className="status-pill paid">Aktif</span>
                    )}
                  </td>
                </tr>
              ))}
              {filteredUnits.length === 0 && !loading && (
                <tr>
                  <td colSpan={8} style={{ textAlign: 'center', padding: 24, color: 'var(--text-dim)' }}>
                    Kriterlere uygun daire bulunamadı.
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
