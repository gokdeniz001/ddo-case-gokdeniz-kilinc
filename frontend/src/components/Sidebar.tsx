import React from 'react';
import {
  Building2,
  TrendingUp,
  Calculator,
  CreditCard,
  Receipt,
  BookOpen,
  Award,
} from 'lucide-react';
import { User } from '../services/api';

interface SidebarProps {
  activeTab: string;
  setActiveTab: (tab: string) => void;
  currentUser: User | null;
  onSwitchRole: (email: string) => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  activeTab,
  setActiveTab,
  currentUser,
  onSwitchRole,
}) => {
  const menuItems = [
    { id: 'community', label: 'Duyuru, Karar & Bakım (F-5)', icon: BookOpen },
    { id: 'dashboard', label: 'Genel Bakış', icon: TrendingUp },
    { id: 'accrual', label: 'Tahakkuk & Dağıtım (T-1)', icon: Calculator },
    { id: 'payments', label: 'Tahsilat & Dekont Eşleme (T-2)', icon: CreditCard },
    { id: 'units', label: 'Daireler & Sakinler (F-1)', icon: Building2 },
    { id: 'expenses', label: 'Gider Yönetimi (F-4)', icon: Receipt },
    { id: 'accounting', label: 'Çift Taraflı Muhasebe (T-4)', icon: BookOpen },
    { id: 'certificates', label: 'Borcu Yoktur Belgesi (T-4)', icon: Award },
  ];

  return (
    <aside className="sidebar">
      {/* Brand */}
      <div className="brand">
        <div className="brand-icon">
          <Building2 color="#ffffff" size={24} />
        </div>
        <div className="brand-text">
          <h2>SiteYönet</h2>
          <span>Enterprise Suite</span>
        </div>
      </div>

      {/* Nav Menu */}
      <nav className="nav-menu">
        {menuItems.filter(item => !['owner','tenant'].includes(currentUser?.role || '') || ['dashboard','units','community','certificates'].includes(item.id)).map((item) => {
          const Icon = item.icon;
          return (
            <button
              key={item.id}
              className={`nav-item ${activeTab === item.id ? 'active' : ''}`}
              onClick={() => setActiveTab(item.id)}
            >
              <Icon size={18} /> {item.label}
            </button>
          );
        })}
      </nav>

      {/* User Info & Role Switcher */}
      <div className="user-profile">
        <div style={{ marginBottom: 12 }}>
          <div style={{ fontSize: '0.85rem', fontWeight: 600, color: '#fff' }}>
            {currentUser?.full_name || 'Giriş Yapılmadı'}
          </div>
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
            {currentUser?.email || 'guest@siteyonet.com'}
          </div>
          <div style={{ marginTop: 4 }}>
            <span className={`role-badge ${currentUser?.role || ''}`}>
              {currentUser?.role?.toUpperCase() || 'GUEST'}
            </span>
          </div>
        </div>

        <div style={{ fontSize: '0.75rem', color: 'var(--text-dim)', marginBottom: 6, fontWeight: 600 }}>
          HIZLI DEMO ROL DEĞİŞTİR:
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 6 }}>
          <button
            className="btn btn-secondary"
            style={{ padding: '6px 8px', fontSize: '0.75rem' }}
            onClick={() => onSwitchRole('yonetici@siteyonet.com')}
          >
            👑 Yönetici
          </button>
          <button
            className="btn btn-secondary"
            style={{ padding: '6px 8px', fontSize: '0.75rem' }}
            onClick={() => onSwitchRole('muhasebe@siteyonet.com')}
          >
            💼 Muhasebe
          </button>
          <button
            className="btn btn-secondary"
            style={{ padding: '6px 8px', fontSize: '0.75rem' }}
            onClick={() => onSwitchRole('denetim@siteyonet.com')}
          >
            🔍 Denetçi
          </button>
          <button
            className="btn btn-secondary"
            style={{ padding: '6px 8px', fontSize: '0.75rem' }}
            onClick={() => onSwitchRole('malik@siteyonet.com')}
          >
            👤 Daire Maliki
          </button>
          <button className="btn btn-secondary" style={{ padding: '6px 8px', fontSize: '0.75rem' }} onClick={() => onSwitchRole('kiraci@siteyonet.com')}>🏠 Kiracı</button>
        </div>
      </div>
    </aside>
  );
};
