// API Service Layer for SiteYonet Frontend

export interface User {
  id: string;
  email: string;
  full_name: string;
  role: 'superadmin' | 'manager' | 'accountant' | 'auditor' | 'owner' | 'tenant';
  site_id?: string;
}

export interface Site {
  id: string;
  name: string;
  code: string;
  address?: string;
  city: string;
  district?: string;
  bank_name?: string;
  bank_iban?: string;
}

export interface Unit {
  id: string;
  block_name: string;
  unit_number: string;
  floor_number: number;
  land_share: number;
  gross_area_m2: number;
  resident_count: number;
  is_exempt: boolean;
  owner_name?: string;
  tenant_name?: string;
}

export interface Debt {
  id: string;
  unit_number: string;
  block_name: string;
  user_name: string;
  title: string;
  debt_type: string;
  term_month: string;
  principal_cents: number;
  paid_principal_cents: number;
  paid_interest_cents: number;
  remaining_cents: number;
  due_date: string;
  status: 'UNPAID' | 'PARTIAL' | 'PAID' | 'CANCELLED' | 'RESTRUCTURED';
}

export interface Expense {
  id: string;
  category: string;
  title: string;
  vendor_name: string;
  invoice_number?: string;
  invoice_date: string;
  amount_cents: number;
  status: string;
}

export interface FinancialSummary {
  total_accruals_cents: number;
  total_collected_cents: number;
  total_expenses_cents: number;
  total_outstanding_debt_cents: number;
  net_cash_flow_cents: number;
  collection_rate_percent: number;
}

export interface ClearanceCertificate {
  id: string;
  unit_number: string;
  block_name: string;
  user_name: string;
  certificate_number: string;
  verification_code: string;
  sha256_hash: string;
  issued_date: string;
  valid_until: string;
  debt_balance_at_issue_cents: number;
}

export interface BankStatement {
  id: string;
  site_id: string;
  statement_date: string;
  reference_number?: string;
  raw_description: string;
  amount_cents: number;
  matched_unit_id?: string;
  matched_unit_no?: string;
  match_confidence?: number;
  match_status: string;
  payment_id?: string;
  created_at: string;
}

const API_BASE = '/api/v1';

export function getStoredToken(): string | null {
  return localStorage.getItem('siteyonet_token');
}

export function setStoredToken(token: string) {
  localStorage.setItem('siteyonet_token', token);
}

export function clearStoredToken() {
  localStorage.removeItem('siteyonet_token');
  localStorage.removeItem('siteyonet_refresh');
  localStorage.removeItem('siteyonet_user');
}

let refreshPending: Promise<string | null> | null = null;
function refreshAccess(): Promise<string | null> {
  if (refreshPending) return refreshPending;
  refreshPending = (async () => {
    const refresh = localStorage.getItem('siteyonet_refresh');
    if (!refresh) return null;
    try {
      const response=await fetch(`${API_BASE}/auth/refresh`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({refresh_token:refresh})});
      if(!response.ok) return null;
      const session=await response.json();setStoredToken(session.token);localStorage.setItem('siteyonet_refresh',session.refresh_token);
      return session.token as string;
    } catch {return null;}
  })().finally(()=>{refreshPending=null;});
  return refreshPending;
}

export async function apiRequest<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> || {}),
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  let res = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers,
  });

  if (res.status === 401 && !endpoint.startsWith('/auth/')) {
    const token = await refreshAccess();
    if (token) {headers.Authorization=`Bearer ${token}`;res=await fetch(`${API_BASE}${endpoint}`,{...options,headers});}
  }
  if (!res.ok) {
    let errMessage = 'İşlem başarısız oldu';
    try {
      const errData = await res.json();
      if (errData?.error?.message) {
        errMessage = errData.error.message;
      }
    } catch {
      // ignore
    }
    throw new Error(errMessage);
  }

  return res.json();
}

export const api = {
  // Auth
  login: async (email: string, pass: string = 'password123') => {
    const res = await apiRequest<{ token: string; refresh_token: string; user: User; site?: Site }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password: pass }),
    });
    setStoredToken(res.token);
    localStorage.setItem('siteyonet_refresh',res.refresh_token);
    localStorage.setItem('siteyonet_user', JSON.stringify(res.user));
    return res;
  },

  getMe: () => apiRequest<User>('/auth/me'),

  // Sites, Units
  getSites: () => apiRequest<Site[]>('/sites'),
  getUnits: () => apiRequest<Unit[]>('/units'),

  // Debts & Payments (T-2)
  getDebts: async (unitId?: string) => {
    const all: Debt[] = []; let after = '';
    while (true) {
      const query = new URLSearchParams({limit:'500'});
      if (unitId) query.set('unit_id',unitId);
      if (after) query.set('after',after);
      const page=await apiRequest<Debt[]>(`/debts?${query}`);all.push(...page);
      if(page.length<500) return all;
      const last=page[page.length-1];after=`${last.due_date.slice(0,10)},${last.id}`;
    }
  },
  processPayment: (payload: {
    unit_id: string;
    amount_cents: number;
    payment_channel: string;
    bank_reference: string;
    description: string;
    payment_date: string;
    idempotency_key?: string;
  }) => apiRequest('/payments', { method: 'POST', headers: { 'Idempotency-Key': payload.idempotency_key || crypto.randomUUID() }, body: JSON.stringify(payload) }),

  fuzzyMatchStatement: (rawDescription: string) =>
    apiRequest<{ unit_number: string; block_name: string; score: number; unit_id: string }>('/bank-statements/match', {
      method: 'POST',
      body: JSON.stringify({ raw_description: rawDescription }),
    }),

  getBankStatements: () => apiRequest<BankStatement[]>('/bank-statements'),

  // Accruals (T-1)
  getDuesDefinitions: () => apiRequest<any[]>('/dues-definitions'),
  getAccruals: () => apiRequest<any[]>('/accruals'),
  createAccrual: (payload: {
    period_id: string;
    title: string;
    term_month: string;
    distribution_type: string;
    total_amount_cents: number;
    due_day: number;
  }) => apiRequest('/accruals', { method: 'POST', body: JSON.stringify(payload) }),

  // Expenses
  getExpenses: () => apiRequest<Expense[]>('/expenses'),
  createExpense: (payload: any) => apiRequest('/expenses', { method: 'POST', body: JSON.stringify(payload) }),

  // Announcements & Maintenance
  getAnnouncements: () => apiRequest<any[]>('/announcements'),
  createAnnouncement: (payload: any) => apiRequest('/announcements', { method: 'POST', body: JSON.stringify(payload) }),
  getMaintenanceRequests: () => apiRequest<any[]>('/maintenance'),
  createMaintenanceRequest: (payload: any) => apiRequest('/maintenance', { method: 'POST', body: JSON.stringify(payload) }),

  // Accounting (T-4)
  getPeriods: () => apiRequest<any[]>('/periods'),
  closePeriod: (id: string) => apiRequest(`/periods/${id}/close`, { method: 'POST' }),
  getLedgerAccounts: () => apiRequest<any[]>('/ledger-accounts'),
  getJournalEntries: () => apiRequest<any[]>('/journal-entries'),
  createJournalEntry: (payload: any) => apiRequest('/journal-entries', { method: 'POST', body: JSON.stringify(payload) }),

  // Reports & Certificates (T-4)
  getFinancialSummary: () => apiRequest<FinancialSummary>('/reports/summary'),
  getBudgetComparison: () => apiRequest<any[]>('/reports/budget-comparison'),
  getCertificates: () => apiRequest<ClearanceCertificate[]>('/certificates'),
  issueCertificate: (unitId: string) =>
    apiRequest<ClearanceCertificate>('/certificates', {
      method: 'POST',
      body: JSON.stringify({ unit_id: unitId, valid_days: 30 }),
    }),
  verifyCertificate: async (code: string) => {
    const res = await fetch(`/api/v1/certificates/verify/${code}`);
    return res.json();
  },
};

export function formatCents(cents: number): string {
  const sign = cents < 0 ? '-' : '';
  const absolute = Math.abs(cents);
  const liras = Math.floor(absolute / 100);
  const krs = absolute % 100;
  return `${sign}${liras.toLocaleString('tr-TR')},${krs.toString().padStart(2, '0')} ₺`;
}

export function normalizeTurkish(text: string): string {
  if (!text) return '';
  return text
    .toLocaleLowerCase('tr-TR')
    .replace(/ğ/g, 'g')
    .replace(/ü/g, 'u')
    .replace(/ş/g, 's')
    .replace(/ı/g, 'i')
    .replace(/ö/g, 'o')
    .replace(/ç/g, 'c')
    .trim();
}
