// k6 load test: contract invocations feed (issue #551).
//
// Exercises GET /api/v1/contracts/{id}/invocations with the status filter that
// drives the "show only failures" dashboard view. Run it directly:
//
//   k6 run tests/load/k6/invocations.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, params, smokeOptions, discoverContract } from './common.js';

export const options = smokeOptions;

const LIMIT = parseInt(__ENV.INVOCATIONS_LIMIT || '50', 10);

export function setup() {
  return { contractId: discoverContract() };
}

export default function (data) {
  // Alternate the status filter so both the unfiltered and the indexed
  // (contract_id, status) query paths are exercised.
  const status = Math.random() < 0.5 ? '' : '&status=FAILED';
  const url = `${BASE_URL}/api/v1/contracts/${data.contractId}/invocations?limit=${LIMIT}${status}`;
  const res = http.get(url, {
    ...params,
    tags: { name: 'listContractInvocations' },
  });
  check(res, {
    'invocations status is 200': (r) => r.status === 200,
    'invocations payload has array': (r) => {
      try {
        return Array.isArray(JSON.parse(r.body).invocations);
      } catch (e) {
        return false;
      }
    },
  });
  sleep(1);
}
