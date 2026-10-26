import React from 'react';
import { ShieldCheck, RefreshCw } from 'lucide-react';
import { Site } from '../services/api';

interface TopbarProps {
  currentSite: Site | null;
  onRefresh: () => void;
  loading?: boolean;
  sites?: Site[];
  onSwitchSite?: (id: string) => void;
}

export const Topbar: React.FC<TopbarProps> = ({ currentSite, onRefresh, loading, sites, onSwitchSite }) => {
  return (
    <header className="topbar">
      <div className="topbar-site-select">
        <ShieldCheck size={20} color="var(--primary)" />
        {sites && sites.length > 1 ? <select aria-label="Aktif site" className="input-field" value={currentSite?.id || ''} onChange={e => onSwitchSite?.(e.target.value)} disabled={loading}>
          {sites.map(site => <option key={site.id} value={site.id}>{site.name}</option>)}
        </select> : <span>{currentSite?.name || 'Site yükleniyor'}</span>}
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
