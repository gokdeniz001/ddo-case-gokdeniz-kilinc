import React, { useEffect, useState } from 'react';
import { api, apiRequest, User } from '../services/api';

export function CommunityPage({ currentUser, onNotify }: { currentUser: User | null; onNotify: (m: { type: 'success' | 'error'; text: string }) => void }) {
  const [kind, setKind] = useState('announcements');
  const [rows, setRows] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [saving, setSaving] = useState(false);
  const canManage = ['manager', 'accountant', 'superadmin'].includes(currentUser?.role || '');
  const canCreate = (canManage && (kind !== 'decisions' || currentUser?.role !== 'accountant')) || (kind === 'maintenance' && ['owner', 'tenant'].includes(currentUser?.role || ''));
  const load = async () => {
    setLoading(true); setError('');
    try { setRows(await apiRequest<any[]>(`/${kind}`)); }
    catch (e: any) { setError(e.message); }
    finally { setLoading(false); }
  };
  useEffect(() => { load(); }, [kind]);
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setSaving(true);
    try {
      const payload: any = { title, content, description: content };
      if (kind === 'decisions') {
        payload.decision_number = `K-${Date.now()}`;
        payload.decision_date = new Date().toISOString().slice(0, 10);
        payload.vote_summary = 'Karar kaydı';
      }
      if (kind === 'maintenance') {
        const units = await api.getUnits();
        if (units.length) payload.unit_id = units[0].id;
      }
      await apiRequest(`/${kind}`, { method: 'POST', body: JSON.stringify(payload) });
      setTitle(''); setContent(''); await load();
      onNotify({ type: 'success', text: 'Kayıt oluşturuldu' });
    } catch (e: any) { onNotify({ type: 'error', text: e.message }); }
    finally { setSaving(false); }
  };
  return <div className="page-container">
    <h1>Duyurular, Kararlar ve Bakım Talepleri</h1>
    <div className="filter-pills-row" style={{ margin: '20px 0' }}>
      {Object.entries({ announcements: 'Duyurular', decisions: 'Kararlar', maintenance: 'Bakım Talepleri' }).map(([id, label]) =>
        <button key={id} className={`filter-pill-btn ${kind === id ? 'active' : ''}`} onClick={() => setKind(id)}>{label}</button>)}
    </div>
    {loading ? <p role="status">Yükleniyor…</p> : error ? <p role="alert">{error} <button onClick={load}>Tekrar dene</button></p> :
      <div className="glass-card" style={{ padding: 20 }}>
        {rows.length === 0 ? <p>Henüz kayıt bulunmuyor.</p> : rows.map(row => <article key={row.id} style={{ padding: '12px 0', borderBottom: '1px solid var(--border-glass)' }}>
          <h3>{row.title}</h3><p>{row.content || row.description}</p>
          {row.decision_number && <small>{row.decision_number} · {row.decision_date?.slice(0, 10)}</small>}
          {row.status && <p>Durum: {row.status}</p>}
          {kind === 'maintenance' && canManage && <button className="btn btn-secondary" onClick={async () => {
            try {
              await apiRequest(`/maintenance/${row.id}/status`, { method: 'PUT', body: JSON.stringify({ status: row.status === 'OPEN' ? 'IN_PROGRESS' : 'RESOLVED' }) });
              await load();
            } catch (e: any) { onNotify({ type: 'error', text: e.message }); }
          }}>Sonraki duruma geçir</button>}
        </article>)}
      </div>}
    {canCreate && <form onSubmit={save} className="glass-card community-form" style={{ marginTop: 20, padding: 20 }}>
      <h3>Yeni kayıt</h3>
      <label>Başlık<input className="input-field" required value={title} onChange={e => setTitle(e.target.value)} /></label>
      <label>İçerik<textarea className="input-field" required value={content} onChange={e => setContent(e.target.value)} /></label>
      <button className="btn btn-primary" disabled={saving}>{saving ? 'Kaydediliyor…' : 'Kaydet'}</button>
    </form>}
  </div>;
}
