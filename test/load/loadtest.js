// Load Test para GOTH Stack
// Executar com: k6 run --duration 5m --vus 100 loadtest.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const loginDuration = new Trend('login_duration');
const dashboardDuration = new Trend('dashboard_duration');

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 20 },   // Ramp up to 20 users
    { duration: '1m', target: 50 },    // Ramp up to 50 users
    { duration: '2m', target: 100 },   // Ramp up to 100 users (peak)
    { duration: '1m', target: 50 },    // Ramp down to 50 users
    { duration: '30s', target: 0 },    // Ramp down to 0
  ],
  thresholds: {
    http_req_duration: ['p(50)<500', 'p(90)<1000', 'p(99)<2000'], // p50 < 500ms, p90 < 1s, p99 < 2s
    http_req_failed: ['rate<0.05'], // Error rate < 5%
    errors: ['rate<0.05'],          // Custom error rate < 5%
    login_duration: ['p(95)<1000'],  // Login p95 < 1s
    dashboard_duration: ['p(95)<500'], // Dashboard p95 < 500ms
  },
};

// Test environment
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Shared state
let csrfToken = '';
let sessionCookie = '';

export function setup() {
  // Get CSRF token before tests
  const res = http.get(`${BASE_URL}/login`);
  
  const tokenMatch = res.body.match(/name="csrf_token" value="([^"]+)"/);
  csrfToken = tokenMatch ? tokenMatch[1] : '';
  
  return { csrfToken };
}

// Test: Health check endpoint
export function healthCheck() {
  const res = http.get(`${BASE_URL}/health`);
  
  const success = check(res, {
    'health status is 200': (r) => r.status === 200,
    'health response time OK': (r) => r.timings.duration < 100,
  });
  
  errorRate.add(!success);
  sleep(0.5);
}

// Test: Home page
export function homePage() {
  const res = http.get(`${BASE_URL}/`);
  
  check(res, {
    'home status is 200': (r) => r.status === 200,
    'home has content': (r) => r.body.includes('GOTH'),
  });
  
  sleep(1);
}

// Test: Login page (anonymous)
export function loginPage() {
  const res = http.get(`${BASE_URL}/login`);
  
  check(res, {
    'login page status is 200': (r) => r.status === 200,
    'login page has form': (r) => r.body.includes('email'),
  });
  
  sleep(0.5);
}

// Test: Register page (anonymous)
export function registerPage() {
  const res = http.get(`${BASE_URL}/register`);
  
  check(res, {
    'register page status is 200': (r) => r.status === 200,
  });
  
  sleep(0.5);
}

// Test: Login flow
export function loginFlow() {
  const email = `testuser_${__VU}@example.com`;
  const password = 'Test1234!';
  
  // Get CSRF token
  const getRes = http.get(`${BASE_URL}/login`);
  const tokenMatch = getRes.body.match(/name="csrf_token" value="([^"]+)"/);
  const token = tokenMatch ? tokenMatch[1] : '';
  
  // Submit login
  const loginStart = Date.now();
  const postRes = http.post(`${BASE_URL}/login`, {
    email: email,
    password: password,
    csrf_token: token,
  }, {
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded',
    },
    redirects: 0, // Don't follow redirects
  });
  
  loginDuration.add(Date.now() - loginStart);
  
  check(postRes, {
    'login redirect': (r) => r.status === 303 || r.status === 302,
  });
  
  sleep(1);
}

// Test: Dashboard (requires auth - simulated)
export function dashboard() {
  // This would require a valid session
  // For load testing, we can test the endpoint directly
  const start = Date.now();
  const res = http.get(`${BASE_URL}/dashboard`, {
    headers: {
      'Cookie': sessionCookie || '',
    },
    redirects: 0,
  });
  
  dashboardDuration.add(Date.now() - start);
  
  // Accept 302 (redirect to login) for unauthenticated
  check(res, {
    'dashboard accessible or redirect': (r) => r.status === 200 || r.status === 302,
  });
  
  sleep(1);
}

// Test: API metrics endpoint
export function metricsEndpoint() {
  const res = http.get(`${BASE_URL}/metrics`);
  
  check(res, {
    'metrics status is 200': (r) => r.status === 200,
    'metrics has prometheus format': (r) => r.body.includes('# HELP'),
  });
  
  sleep(0.5);
}

// Main load test scenario
export default function () {
  // Mix of different scenarios
  const scenario = Math.random();
  
  if (scenario < 0.3) {
    // 30% - Health checks
    healthCheck();
  } else if (scenario < 0.5) {
    // 20% - Home page
    homePage();
  } else if (scenario < 0.7) {
    // 20% - Login page
    loginPage();
  } else if (scenario < 0.85) {
    // 15% - Register page
    registerPage();
  } else if (scenario < 0.95) {
    // 10% - Login flow
    loginFlow();
  } else {
    // 5% - Metrics
    metricsEndpoint();
  }
}

// Handle test completion
export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    './test-results/load-test-summary.json': JSON.stringify(data),
  };
}

function textSummary(data, options) {
  const { indent = '', enableColors = false } = options;
  
  let summary = '\n=== Load Test Summary ===\n\n';
  
  // HTTP stats
  summary += `${indent}HTTP Requests: ${data.metrics.http_reqs.values.count}\n`;
  summary += `${indent}HTTP Failures: ${data.metrics.http_req_failed.values.rate * 100}%\n`;
  summary += `${indent}Avg Duration: ${data.metrics.http_req_duration.values.avg.toFixed(2)}ms\n`;
  summary += `${indent}p95 Duration: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms\n`;
  summary += `${indent}p99 Duration: ${data.metrics.http_req_duration.values['p(99)'].toFixed(2)}ms\n`;
  
  // Custom metrics
  if (data.metrics.errors) {
    summary += `${indent}Custom Error Rate: ${data.metrics.errors.values.rate * 100}%\n`;
  }
  
  // VUs
  summary += `\n${indent}Max VUs: ${data.metrics.vus_max.values.value}\n`;
  summary += `${indent}Test Duration: ${data.state.testRunDurationMs / 1000}s\n`;
  
  // Thresholds
  summary += '\n--- Thresholds ---\n';
  for (const [name, threshold] of Object.entries(data.metrics)) {
    if (threshold.thresholds) {
      for (const [tName, tResult] of Object.entries(threshold.thresholds)) {
        const status = tResult.ok ? '✓' : '✗';
        summary += `${indent}${status} ${name}: ${tName}\n`;
      }
    }
  }
  
  return summary;
}
