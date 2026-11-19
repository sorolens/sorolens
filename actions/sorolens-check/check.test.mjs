import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { runCheck } from './check.mjs';

function mockFetch(responses) {
  return async (url) => {
    const matched = Object.entries(responses).find(([pattern]) => url.includes(pattern));
    if (!matched) {
      return {
        ok: false,
        status: 404,
        statusText: 'Not Found',
        json: async () => ({ error: 'Not Found' }),
      };
    }
    const [, res] = matched;
    return {
      ok: res.status ? res.status >= 200 && res.status < 300 : true,
      status: res.status ?? 200,
      statusText: res.statusText ?? 'OK',
      json: async () => res.body,
    };
  };
}

describe('sorolens-check action', () => {
  const contractId = 'CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5';

  test('passes when contract TTL and health score exceed thresholds', async () => {
    const fetchImpl = mockFetch({
      '/stats': {
        body: { last_synced_ledger: 1000, storage_count: 2 },
      },
      '/storage': {
        body: {
          storage: [
            { key_xdr: 'AAAA1111', durability: 'persistent', ledgers_until_expiry: 500 },
            { key_xdr: 'AAAA2222', durability: 'instance', ledgers_until_expiry: 800 },
          ],
          next_cursor: '',
        },
      },
      '/health-score': {
        body: { contract_id: contractId, score: 92 },
      },
    });

    const logs = [];
    const outputs = await runCheck({
      contractId,
      apiUrl: 'https://api.sorolens.com',
      minTtlLedgers: 400,
      minHealthScore: 85,
      fetchImpl,
      log: (msg) => logs.push(msg),
    });

    assert.equal(outputs.status, 'passed');
    assert.equal(outputs['health-score'], '92');
    assert.equal(outputs['min-ttl'], '500');
    assert.equal(outputs['storage-count'], '2');
  });

  test('fails when storage TTL is below min-ttl-ledgers threshold', async () => {
    const fetchImpl = mockFetch({
      '/stats': {
        body: { last_synced_ledger: 1000 },
      },
      '/storage': {
        body: {
          storage: [
            { key_xdr: 'AAAA1111', durability: 'persistent', ledgers_until_expiry: 150 },
          ],
          next_cursor: '',
        },
      },
      '/health-score': {
        body: { contract_id: contractId, score: 90 },
      },
    });

    const errors = [];
    await assert.rejects(
      async () => {
        await runCheck({
          contractId,
          minTtlLedgers: 200,
          fetchImpl,
          errorLog: (msg) => errors.push(msg),
        });
      },
      /violat/
    );

    assert.equal(errors.some((e) => e.includes('150 ledgers remaining')), true);
  });

  test('fails when health score is below min-health-score threshold', async () => {
    const fetchImpl = mockFetch({
      '/stats': {
        body: { last_synced_ledger: 1000 },
      },
      '/storage': {
        body: { storage: [], next_cursor: '' },
      },
      '/health-score': {
        body: { contract_id: contractId, score: 65 },
      },
    });

    const errors = [];
    await assert.rejects(
      async () => {
        await runCheck({
          contractId,
          minHealthScore: 80,
          fetchImpl,
          errorLog: (msg) => errors.push(msg),
        });
      },
      /Contract health score is 65, below threshold 80/
    );

    assert.equal(errors.some((e) => e.includes('Contract health score is 65')), true);
  });

  test('throws an error if contractId is missing', async () => {
    await assert.rejects(
      async () => {
        await runCheck({ contractId: '' });
      },
      /contract-id is required/
    );
  });
});
