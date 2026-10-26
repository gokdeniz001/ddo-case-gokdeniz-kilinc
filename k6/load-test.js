import http from 'k6/http';
import { check, sleep } from 'k6';

// Performance load test for Task 1 (Accrual Engine) and Task 2 (Payment Allocation)
export const options = {
  stages: [
    { duration: '15s', target: 20 },  // Ramp-up to 20 virtual users
    { duration: '30s', target: 50 },  // Sustained load at 50 virtual users
    { duration: '15s', target: 0 },   // Ramp-down
  ],
  thresholds: {
    http_req_duration: ['p(95)<250'], // 95% of requests must complete below 250ms
    http_req_failed: ['rate<0.01'],    // Error rate must be less than 1%
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

export function setup() {
  // Login as manager to obtain JWT
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    email: 'yonetici@siteyonet.com',
    password: 'password123',
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  const body = JSON.parse(loginRes.body);
  return { token: body.token };
}

export default function (data) {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  // 1. Health check & DDO Header verification
  const healthRes = http.get(`${BASE_URL}/health`);
  check(healthRes, {
    'health status is 200': (r) => r.status === 200,
    'has X-DDO-Build header': (r) => r.headers['X-Ddo-Build'] === '0c7cc1',
  });

  // 2. Query Debts & Financial summary (Simulating concurrent users)
  const debtsRes = http.get(`${BASE_URL}/api/v1/debts`, { headers });
  check(debtsRes, {
    'debts status is 200': (r) => r.status === 200,
  });

  // 3. Test T-2 Fuzzy match throughput
  const fuzzyRes = http.post(`${BASE_URL}/api/v1/bank-statements/match`, JSON.stringify({
    raw_description: 'A BLOK DAIRE: 5 AHMET YILMAZ AIDAT',
  }), { headers });

  check(fuzzyRes, {
    'fuzzy match is 200': (r) => r.status === 200,
  });

  sleep(0.5);
}
