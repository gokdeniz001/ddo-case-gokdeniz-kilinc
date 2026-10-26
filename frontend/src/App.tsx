import React, { useState, useEffect } from 'react';
import { CheckCircle2, AlertCircle, X } from 'lucide-react';
import { api, User, Site } from './services/api';
import { Sidebar } from './components/Sidebar';
import { Topbar } from './components/Topbar';
import { DashboardPage } from './pages/DashboardPage';
import { AccrualsPage } from './pages/AccrualsPage';
import { PaymentsPage } from './pages/PaymentsPage';
import { UnitsPage } from './pages/UnitsPage';
import { ExpensesPage } from './pages/ExpensesPage';
import { AccountingPage } from './pages/AccountingPage';
import { CertificatesPage } from './pages/CertificatesPage';

export default function App() {
  const [activeTab, setActiveTab] = useState('dashboard');
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [currentSite, setCurrentSite] = useState<Site | null>(null);
  const [loading, setLoading] = useState(false);
  const [statusMsg, setStatusMsg] = useState<{ type: 'success' | 'error'; text: string; id: number } | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  const notify = (msg: { type: 'success' | 'error'; text: string }) => {
    setStatusMsg({ ...msg, id: Date.now() });
  };

  // Auto-dismiss notification after 4.5 seconds
  useEffect(() => {
    if (!statusMsg) return;
    const timer = setTimeout(() => {
      setStatusMsg(null);
    }, 4500);
    return () => clearTimeout(timer);
  }, [statusMsg]);

  // Switch demo account role
  const switchRole = async (email: string) => {
    setLoading(true);
    setStatusMsg(null);
    try {
      const res = await api.login(email);
      setCurrentUser(res.user);
      if (res.site) setCurrentSite(res.site);
      setRefreshKey((prev) => prev + 1);
      notify({
        type: 'success',
        text: `Rol değiştirildi: ${res.user.full_name} (${res.user.role})`,
      });
    } catch (err: any) {
      notify({ type: 'error', text: err.message || 'Giriş yapılamadı' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    // Default login as Manager on startup
    switchRole('yonetici@siteyonet.com');
  }, []);

  const handleGlobalRefresh = () => {
    setRefreshKey((prev) => prev + 1);
    notify({ type: 'success', text: 'Sayfa verileri canlı olarak tazelendi' });
  };

  return (
    <div className="app-container">
      {/* 1. Sidebar Component */}
      <Sidebar
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        currentUser={currentUser}
        onSwitchRole={switchRole}
      />

      {/* 2. Main Wrapper */}
      <main className="main-wrapper">
        {/* Topbar Component */}
        <Topbar
          currentSite={currentSite}
          onRefresh={handleGlobalRefresh}
          loading={loading}
        />

        {/* Content Body */}
        <div className="content-body" key={refreshKey}>
          {/* Toast Notification Banner with Auto-dismiss */}
          {statusMsg && (
            <div key={statusMsg.id} className={`toast-banner ${statusMsg.type}`}>
              <div className="toast-banner-content">
                {statusMsg.type === 'success' ? (
                  <CheckCircle2 size={18} style={{ flexShrink: 0 }} />
                ) : (
                  <AlertCircle size={18} style={{ flexShrink: 0 }} />
                )}
                <span>{statusMsg.text}</span>
              </div>
              <button
                type="button"
                className="toast-close-btn"
                onClick={() => setStatusMsg(null)}
                title="Kapat"
              >
                <X size={16} />
              </button>
              <div className="toast-progress-bar" />
            </div>
          )}

          {/* Dynamic SPA Page Rendering */}
          {activeTab === 'dashboard' && <DashboardPage onNotify={notify} />}
          {activeTab === 'accrual' && <AccrualsPage onNotify={notify} />}
          {activeTab === 'payments' && <PaymentsPage onNotify={notify} />}
          {activeTab === 'units' && <UnitsPage onNotify={notify} />}
          {activeTab === 'expenses' && <ExpensesPage onNotify={notify} />}
          {activeTab === 'accounting' && <AccountingPage onNotify={notify} />}
          {activeTab === 'certificates' && <CertificatesPage onNotify={notify} />}
        </div>
      </main>
    </div>
  );
}
