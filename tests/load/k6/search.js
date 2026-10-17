// k6 load test: contract search (issue #551).
//
// Exercises GET /api/v1/search?q=... The default query matches the label
// pattern produced by cmd/seedload ("load-contract-N"); override with
// SEARCH_QUERY for a real target. Run it directly:
//
//   k6 run tests/load/k6/search.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, params, smokeOptions } from './common.js';

export const options = smokeOptions;

const QUERY = __ENV.SEARCH_QUERY || 'load-contract';
const LIMIT = parseInt(__ENV.SEARCH_LIMIT || '10', 10);

export default function () {
  const url = `${BASE_URL}/api/v1/search?q=${encodeURIComponent(QUERY)}&limit=${LIMIT}`;
  const res = http.get(url, {
    ...params,
    tags: { name: 'searchContracts' },
  });
  check(res, {
    'search status is 200': (r) => r.status === 200,
    'search payload has items': (r) => {
      try {
        return Array.isArray(JSON.parse(r.body).items);
      } catch (e) {
        return false;
      }
    },
  });
  sleep(1);
}
