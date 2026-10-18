// Shared configuration and helpers for the k6 load scripts (issue #551).
//
// All scripts target the Sorolens REST read surface, which is anonymous by
// default (see apps/api/internal/router/router.go: read routes pass without a
// credential), so no API key is required for a smoke run. Set API_KEY to also
// exercise the authenticated path.
//
// Everything is env-tunable so the same scripts serve a quick CI smoke run and
// a large local benchmark:
//
//   BASE_URL          target API base (default http://localhost:8080)
//   API_KEY           optional Bearer token
//   CONTRACT_ID       optional contract to target; when unset the scripts
//                     discover one via GET /api/v1/contracts
//   SEARCH_QUERY      optional search term; when unset a label prefix is used
import http from 'k6/http';
import { check } from 'k6';

export const BASE_URL = (__ENV.BASE_URL || 'http://localhost:8080').replace(/\/$/, '');

const headers = { Accept: 'application/json' };
if (__ENV.API_KEY) {
  headers.Authorization = `Bearer ${__ENV.API_KEY}`;
}

export const params = { headers };

// Latency/error budgets. Defaults are generous enough for a small seeded smoke
// dataset on CI runners, while still failing on an outright outage or a gross
// regression. Override with env when running a real benchmark.
const P95_MS = parseInt(__ENV.P95_MS || '1000', 10);
const MAX_FAIL_RATE = __ENV.MAX_FAIL_RATE || '0.01';

export const THRESHOLDS = {
  http_req_failed: [`rate<${MAX_FAIL_RATE}`],
  http_req_duration: [`p(95)<${P95_MS}`],
};

// Options for a CI smoke run: tiny load, short duration, thresholds enforced.
export const smokeOptions = {
  vus: parseInt(__ENV.VUS || '10', 10),
  duration: __ENV.DURATION || '30s',
  thresholds: THRESHOLDS,
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'rate'],
};

// discoverContract returns a contract id to exercise the per-contract read
// endpoints. It prefers an explicit CONTRACT_ID and otherwise reads the first
// id from the list endpoint (newest page).
export function discoverContract() {
  if (__ENV.CONTRACT_ID) {
    return __ENV.CONTRACT_ID;
  }
  const url = `${BASE_URL}/api/v1/contracts?limit=1`;
  const res = http.get(url, params);
  const ok = check(res, {
    'contract discovery returned 200': (r) => r.status === 200,
  });
  if (!ok) {
    throw new Error(
      `could not discover a contract: GET ${url} -> HTTP ${res.status}`,
    );
  }
  const body = res.json();
  const contracts = body && body.contracts;
  if (!contracts || contracts.length === 0) {
    throw new Error('no contracts present; run the seed script first');
  }
  return contracts[0].id;
}
