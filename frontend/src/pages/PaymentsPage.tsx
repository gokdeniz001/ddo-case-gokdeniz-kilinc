import React, { useEffect, useState } from 'react';
import { CreditCard, Search, CheckCircle2, Clock } from 'lucide-react';
import { api, apiRequest, formatCents, BankStatement } from '../services/api';

interface PaymentsPageProps {
  readOnly?: boolean;
  onNotify: (msg: { type: 'success' | 'error'; text: string }) => void;
}

export const PaymentsPage: React.FC<PaymentsPageProps> = ({ onNotify, readOnly }) => {
  const [units, setUnits] = useState<any[]>([]);
  const [paymentUnit, setPaymentUnit] = useState('');
  const [amount, setAmount] = useState('');
  const [saving, setSaving] = useState(false);
  const [paymentKey, setPaymentKey] = useState(() => crypto.randomUUID());
  useEffect(() => {api.getUnits().then(setUnits).catch(e=>onNotify({type:'error',text:e.message}));}, []);
  const [bankDescInput, setBankDescInput] = useState('');
  const [matchResult, setMatchResult] = useState<any>(null);
  const [matching, setMatching] = useState(false);
  const [bankStatements, setBankStatements] = useState<BankStatement[]>([]);

  useEffect(() => {
    api.getBankStatements().then((list) => setBankStatements(list || [])).catch(() => {});
  }, []);

  const handleFuzzyMatch = async () => {
    if (!bankDescInput.trim()) {
      onNotify({ type: 'error', text: 'Lütfen önce bir daire seçin veya dekont metni girin.' });
      return;
    }
    try {
      setMatching(true);
      const res = await api.fuzzyMatchStatement(bankDescInput);
      setMatchResult(res);
      setPaymentUnit(res.unit_id);
      onNotify({
        type: 'success',
        text: `Eşleşme bulundu: Daire ${res.unit_number} (%${Math.round(res.score * 100)} güven skoru)`,
      });
    } catch (err: any) {
      setMatchResult(null);
      onNotify({ type: 'error', text: err.message || 'Eşleşen daire bulunamadı' });
    } finally {
      setMatching(false);
    }
  };

  return (
    <div className="page-container">
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: '1.75rem', marginBottom: 6 }}>Tahsilat & Dekont Eşleme (Task 2)</h1>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.95rem' }}>
          Şelale Dağıtım Kuralı (Önce Gecikme Zammı → Anapara → FIFO) ve Bulanık Metin Eşleştirme Motoru.
        </p>
      </div>

      {!readOnly && <form className="glass-card community-form" style={{padding:20,marginBottom:24}} onSubmit={async e=>{
        e.preventDefault();setSaving(true);
        try {
          const [whole,fraction='']=amount.replace(',','.').split('.');
          if(!/^\d+$/.test(whole)||!/^\d{0,2}$/.test(fraction)) throw new Error('Tutar en fazla iki ondalık basamak içermeli');
          const cents=Number(whole)*100+Number(fraction.padEnd(2,'0'));
          if(!Number.isSafeInteger(cents)||cents<=0)throw new Error('Geçersiz tutar');
          const result:any=await api.processPayment({unit_id:paymentUnit,amount_cents:cents,payment_date:new Date().toISOString().slice(0,10),payment_channel:'BANK_TRANSFER',bank_reference:paymentKey,description:'Arayüz tahsilatı',idempotency_key:paymentKey});
          onNotify({type:'success',text:`Tahsilat kaydedildi. Avans: ${formatCents(result.unallocated_cents)}`});setPaymentKey(crypto.randomUUID());setAmount('');
        }catch(e:any){onNotify({type:'error',text:e.message});}finally{setSaving(false);}
      }}>
        <h3>Tahsilat kaydet</h3>
        <label>Daire<select aria-label="Tahsilat dairesi" className="input-field" required value={paymentUnit} onChange={e=>{setPaymentUnit(e.target.value);setPaymentKey(crypto.randomUUID());}}><option value="">Daire seçin</option>{units.filter(u=>u.owner_name||u.tenant_name).map(u=><option key={u.id} value={u.id}>{u.block_name} / {u.unit_number} — {u.owner_name||u.tenant_name}</option>)}</select></label>
        <label>Tutar (TL)<input className="input-field" inputMode="decimal" required value={amount} onChange={e=>{setAmount(e.target.value);setPaymentKey(crypto.randomUUID());}} placeholder="1500,00" /></label>
        <button className="btn btn-primary" disabled={saving}>{saving?'Kaydediliyor…':'Tahsilatı kaydet'}</button>
        <label>Banka ekstresi CSV içe aktar<input className="input-field" type="file" accept=".csv,text/csv" onChange={async e=>{
          const file=e.target.files?.[0];if(!file)return;
          try {const result:any=await apiRequest('/bank-statements/import',{method:'POST',headers:{'Idempotency-Key':crypto.randomUUID()},body:JSON.stringify({csv:await file.text()})});setBankStatements(await api.getBankStatements());onNotify({type:'success',text:`${result.imported} satır alındı, ${result.matched} daire eşleşti`});}catch(e:any){onNotify({type:'error',text:e.message});}
        }} /></label>
        <small>CSV başlığı: date,reference,description,amount_cents. Eşleşme tahsilatı kendiliğinden kaydetmez; daireyi ve tutarı kontrol ederek yukarıdan kaydedin.</small>
      </form>}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24, marginBottom: 24 }}>
        {/* Fuzzy Matching Simulator */}
        <div className="glass-card" style={{ padding: 24 }}>
          <h3 style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
            <Search size={20} color="var(--primary)" /> Bulanık Dekont Eşleştirme Motoru
          </h3>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            {/* Unit Picker replaces the old textarea-only input */}
            <UnitPickerForStatement onSelect={setBankDescInput} selected={bankDescInput} />

            <button
              className="btn btn-primary"
              style={{ padding: '12px 20px' }}
              onClick={handleFuzzyMatch}
              disabled={matching}
            >
              {matching ? 'Eşleştiriliyor...' : 'Dekontu Analiz Et & Daireyi Bul'}
            </button>

            {matchResult && (
              <div
                style={{
                  padding: 16,
                  borderRadius: 'var(--radius-sm)',
                  background: 'rgba(16, 185, 129, 0.1)',
                  border: '1px solid var(--success)',
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                  <strong style={{ color: 'var(--success)' }}>Eşleşen Daire: {matchResult.unit_number}</strong>
                  <span style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                    Güven Skoru: %{Math.round(matchResult.score * 100)}
                  </span>
                </div>
                <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                  Malik / Sakin: <strong>{matchResult.owner_name}</strong>
                </div>
                <div style={{ fontSize: '0.8rem', color: 'var(--text-dim)', marginTop: 6 }}>
                  Eşleşme Kuralı: Blok & No Regex + Trigram İsim Benzerliği
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Waterfall Allocation Info */}
        <div className="glass-card" style={{ padding: 24 }}>
          <h3 style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
            <Clock size={20} color="var(--warning)" /> Şelale (Waterfall) Dağıtım Sırası
          </h3>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{ padding: '12px 16px', background: 'rgba(255,255,255,0.03)', borderRadius: 8, borderLeft: '3px solid var(--danger)' }}>
              <strong>1. Adım: Gecikme Zammı (Aylık %5 / Günlük Oran)</strong>
              <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginTop: 4 }}>
                Gelen tahsilat öncelikle birikmiş yasal gecikme faizini sıfırlar.
              </p>
            </div>
            <div style={{ padding: '12px 16px', background: 'rgba(255,255,255,0.03)', borderRadius: 8, borderLeft: '3px solid var(--warning)' }}>
              <strong>2. Adım: En Eski Vade Önceliği (FIFO Anapara)</strong>
              <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginTop: 4 }}>
                Kalan tutar vadesi en eski borç kaleminin anaparasını kapatır.
              </p>
            </div>
            <div style={{ padding: '12px 16px', background: 'rgba(255,255,255,0.03)', borderRadius: 8, borderLeft: '3px solid var(--success)' }}>
              <strong>3. Adım: Fazla Ödeme & Avans (340 Hesabı)</strong>
              <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginTop: 4 }}>
                Tüm borçlar kapandıktan sonra artan tutar dairenin avans bakiyesine aktarılır.
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Bank Statements Table */}
      <div className="glass-card" style={{ padding: 24 }}>
        <h3 style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
          <CreditCard size={18} color="var(--primary)" /> Banka Hesap Hareketleri (Ekstre)
        </h3>
        <p style={{ fontSize: '0.82rem', color: 'var(--text-dim)', marginBottom: 12 }}>
          💡 Bir satıra tıklayarak dekontu analiz alanına aktarabilirsiniz.
        </p>

        <div className="table-responsive">
          <table className="data-table">
            <thead>
              <tr>
                <th>TARİH</th>
                <th>REFERANS NO</th>
                <th>AÇIKLAMA</th>
                <th>TUTAR</th>
                <th>EŞLEŞİLEN DAİRE</th>
                <th>DURUM</th>
              </tr>
            </thead>
            <tbody>
              {bankStatements.map((b) => (
                <tr
                  key={b.id}
                  style={{ cursor: 'pointer' }}
                  title="Bu dekontu analiz alanına aktar"
                  onClick={() => setBankDescInput(b.raw_description)}
                >
                  <td style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                    {b.statement_date ? b.statement_date.substring(0, 10) : '—'}
                  </td>
                  <td><span style={{ fontFamily: 'var(--font-mono)', fontSize: '0.85rem' }}>{b.reference_number || '—'}</span></td>
                  <td style={{ maxWidth: 320, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                    {b.raw_description}
                  </td>
                  <td style={{ fontWeight: 600 }}>{formatCents(b.amount_cents)}</td>
                  <td>{b.matched_unit_no ? `Daire ${b.matched_unit_no}` : '—'}</td>
                  <td>
                    <span className={`status-pill ${b.match_status.toLowerCase()}`}>
                      {b.match_status}
                    </span>
                  </td>
                </tr>
              ))}
              {bankStatements.length === 0 && (
                <tr>
                  <td colSpan={6} style={{ textAlign: 'center', padding: 20, color: 'var(--text-dim)' }}>
                    Banka hareketi bulunmuyor.
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

/* ------------------------------------------------------------------ */
/* UnitPickerForStatement                                              */
/* Loads real units from API → block filter pills + clickable box grid */
/* Selecting a unit auto-generates a bank description string           */
/* ------------------------------------------------------------------ */
interface UnitPickerProps {
  onSelect: (description: string) => void;
  selected: string;
}

const UnitPickerForStatement: React.FC<UnitPickerProps> = ({ onSelect, selected }) => {
  const [units, setUnits] = useState<any[]>([]);
  const [blockFilter, setBlockFilter] = useState('ALL');
  const [pickedUnitId, setPickedUnitId] = useState<string | null>(null);

  useEffect(() => {
    api.getUnits().then((data) => setUnits(data || [])).catch(() => {});
  }, []);

  const blocks = ['ALL', ...Array.from(new Set(units.map((u) => u.block_name))).sort()];
  const visible = units.filter((u) => blockFilter === 'ALL' || u.block_name === blockFilter);

  const handlePick = (unit: any) => {
    const isSame = pickedUnitId === unit.id;
    if (isSame) {
      setPickedUnitId(null);
      onSelect('');
    } else {
      setPickedUnitId(unit.id);
      const owner = (unit.owner_name || unit.tenant_name || 'MALIK').toUpperCase();
      onSelect(`${unit.block_name.toUpperCase()} DAIRE: ${unit.unit_number} ${owner} AIDAT`);
    }
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
        <label style={{ fontSize: '0.85rem', color: 'var(--text-muted)', fontWeight: 600 }}>
          Banka Dekont Açıklaması
        </label>
        <span style={{ fontSize: '0.75rem', color: 'var(--text-dim)' }}>
          Daire kutusuna tıkla → açıklama otomatik oluşur
        </span>
      </div>

      {/* Block filter pills */}
      <div className="filter-pills-row" style={{ marginBottom: 10 }}>
        {blocks.map((b) => (
          <button
            key={b}
            type="button"
            className={`filter-pill-btn ${blockFilter === b ? 'active' : ''}`}
            onClick={() => setBlockFilter(b)}
          >
            {b === 'ALL'
              ? `🏢 Tümü (${units.length})`
              : `🏢 ${b} (${units.filter((u) => u.block_name === b).length})`}
          </button>
        ))}
      </div>

      {/* Unit box grid */}
      <div
        className="box-grid"
        style={{
          maxHeight: 150,
          overflowY: 'auto',
          padding: 4,
          border: '1px solid var(--border-glass)',
          borderRadius: 'var(--radius-sm)',
          background: 'rgba(0,0,0,0.2)',
          marginBottom: 10,
        }}
      >
        {visible.length === 0 ? (
          <div style={{ padding: '12px', color: 'var(--text-dim)', fontSize: '0.82rem' }}>
            Daireler yükleniyor…
          </div>
        ) : (
          visible.map((u) => {
            const isSelected = pickedUnitId === u.id;
            return (
              <div
                key={u.id}
                className={`select-box-card ${isSelected ? 'active' : ''}`}
                onClick={() => handlePick(u)}
              >
                <div className="select-box-title">
                  <span>{u.block_name[0]}-{u.unit_number}</span>
                  {isSelected && <CheckCircle2 size={13} color="var(--primary)" />}
                </div>
                <div className="select-box-desc" title={u.owner_name || u.tenant_name || '—'}>
                  {u.owner_name || u.tenant_name || 'Boş'}
                </div>
              </div>
            );
          })
        )}
      </div>

      {/* Textarea — editable after unit selection */}
      <textarea
        className="input-field"
        rows={2}
        value={selected}
        placeholder="Yukarıdan bir daire seçin veya buraya dekont metni yazın…"
        onChange={(e) => onSelect(e.target.value)}
        style={{ resize: 'vertical' }}
      />
    </div>
  );
};
