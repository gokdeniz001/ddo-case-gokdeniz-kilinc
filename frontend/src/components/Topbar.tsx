import React from 'react';
import { ShieldCheck, RefreshCw } from 'lucide-react';
import { Site } from '../services/api';

interface TopbarProps {
  currentSite: Site | null;
  onRefresh: () => void;
  loading?: boolean;
}

export const Topbar: React.FC<TopbarProps> = ({ currentSite, onRefresh, loading }) => {
  return (
    <header className="topbar">
      <div className="topbar-site-select">
        <ShieldCheck size={20} color="var(--primary)" />
        <span style={{ fontWeight: 600, fontSize: '1rem', color: '#fff' }}>
          {currentSite?.name || 'Güneş Park Evleri'}
        </span>
        <span style={{ fontSize: '0.8rem', color: 'var(--text-dim)' }}>
          ({currentSite?.code || 'STE01'})
        </span>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <span
          style={{
            fontSize: '0.8rem',
            padding: '4px 10px',
            background: 'rgba(59, 130, 246, 0.1)',
            border: '1px solid var(--border-glass)',
            borderRadius: 20,
            color: 'var(--primary)',
            fontFamily: 'var(--font-mono)',
          }}
        >
          X-DDO-Build: 0c7cc1
        </span>
        <button
          className="btn btn-secondary"
          onClick={onRefresh}
          disabled={loading}
          title="Verileri Canlı Yenile"
        >
          <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
        </button>
      </div>
    </header>
  );
};
