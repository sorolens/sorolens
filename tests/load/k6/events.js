// k6 load test: contract events feed (issue #551).
//
// Exercises GET /api/v1/contracts/{id}/events — the highest-volume read on the
// API. Run it directly:
//
//   k6 run tests/load/k6/events.js
//
// or with a target/seed already loaded:
//
//   BASE_URL=https://api.sorolens.xyz CONTRACT_ID=CABC... k6 run tests/load/k6/events.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, params, smokeOptions, discoverContract } from './common.js';

export const options = smokeOptions;

const LIMIT = parseInt(__ENV.EVENTS_LIMIT || '50', 10);

export function setup() {
  return { contractId: discoverContract() };
}

export default function (data) {
  const url = `${BASE_URL}/api/v1/contracts/${data.contractId}/events?limit=${LIMIT}`;
  const res = http.get(url, {
    ...params,
    tags: { name: 'listContractEvents' },
  });
  check(res, {
    'events status is 200': (r) => r.status === 200,
    'events payload has array': (r) => {
      try {
        return Array.isArray(JSON.parse(r.body).events);
      } catch (e) {
        return false;
      }
    },
  });
  sleep(1);
}
