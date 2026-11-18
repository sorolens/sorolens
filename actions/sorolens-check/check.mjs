#!/usr/bin/env node

import fs from 'node:fs';

export async function runCheck(options = {}) {
  const contractId = options.contractId ?? process.env.INPUT_CONTRACT_ID ?? process.env.CONTRACT_ID;
  const apiUrl = (options.apiUrl ?? process.env.INPUT_API_URL ?? process.env.API_URL ?? 'https://api.sorolens.com').replace(/\/+$/, '');
  const apiKey = options.apiKey ?? process.env.INPUT_API_KEY ?? process.env.API_KEY ?? '';
  const minTtlLedgers = Number(options.minTtlLedgers ?? process.env.INPUT_MIN_TTL_LEDGERS ?? process.env.MIN_TTL_LEDGERS ?? 0);
  const minHealthScore = Number(options.minHealthScore ?? process.env.INPUT_MIN_HEALTH_SCORE ?? process.env.MIN_HEALTH_SCORE ?? 0);
  let currentLedger = options.currentLedger ?? (process.env.INPUT_CURRENT_LEDGER ? Number(process.env.INPUT_CURRENT_LEDGER) : null);

  const fetchImpl = options.fetchImpl ?? globalThis.fetch;
  const log = options.log ?? console.log;
  const errorLog = options.errorLog ?? console.error;

  if (!contractId) {
    throw new Error('contract-id is required');
  }

  const headers = {
    'Accept': 'application/json',
    'User-Agent': 'sorolens-check-action/1.0.0',
  };
  if (apiKey) {
    headers['X-API-Key'] = apiKey;
    headers['Authorization'] = `Bearer ${apiKey}`;
  }

  log(`[sorolens-check] Checking contract: ${contractId}`);
  log(`[sorolens-check] API URL: ${apiUrl}`);

  // 1. Fetch contract stats
  const statsRes = await fetchImpl(`${apiUrl}/api/v1/contracts/${contractId}/stats`, { headers });
  if (!statsRes.ok) {
    throw new Error(`Failed to fetch contract stats: ${statsRes.status} ${statsRes.statusText}`);
  }
  const stats = await statsRes.json();
  if (currentLedger === null || isNaN(currentLedger)) {
    currentLedger = stats.last_synced_ledger ?? 0;
  }
  log(`[sorolens-check] Current reference ledger: ${currentLedger}`);

  const failures = [];
  let minTtlFound = Infinity;
  let storageCount = 0;
  let healthScore = null;

  // 2. Check storage TTL if minTtlLedgers > 0 or to discover minTtl
  const storageRes = await fetchImpl(`${apiUrl}/api/v1/contracts/${contractId}/storage?limit=200`, { headers });
  if (storageRes.ok) {
    const storageData = await storageRes.json();
    const entries = storageData.storage ?? [];
    storageCount = entries.length;
    log(`[sorolens-check] Retrieved ${storageCount} storage entries`);

    for (const entry of entries) {
      let ledgersRemaining = entry.ledgers_until_expiry;
      if (ledgersRemaining == null && entry.live_until_ledger != null) {
        ledgersRemaining = entry.live_until_ledger - currentLedger;
      }
      if (ledgersRemaining != null) {
        if (ledgersRemaining < minTtlFound) {
          minTtlFound = ledgersRemaining;
        }
        if (minTtlLedgers > 0 && ledgersRemaining < minTtlLedgers) {
          failures.push(`Storage entry (${entry.durability ?? 'unknown'}, key: ${entry.key_xdr?.slice(0, 16)}...) has ${ledgersRemaining} ledgers remaining, below threshold ${minTtlLedgers}`);
        }
      }
    }
  } else if (minTtlLedgers > 0) {
    failures.push(`Failed to fetch contract storage: ${storageRes.status} ${storageRes.statusText}`);
  }

  if (minTtlFound === Infinity) {
    minTtlFound = 0;
  }

  // 3. Check health score if minHealthScore > 0 or for metrics
  const healthRes = await fetchImpl(`${apiUrl}/api/v1/contracts/${contractId}/health-score`, { headers });
  if (healthRes.ok) {
    const healthData = await healthRes.json();
    healthScore = healthData.score ?? healthData.health_score ?? null;
    log(`[sorolens-check] Contract composite health score: ${healthScore}`);
    if (minHealthScore > 0 && (healthScore == null || healthScore < minHealthScore)) {
      failures.push(`Contract health score is ${healthScore ?? 'unknown'}, below threshold ${minHealthScore}`);
    }
  } else if (minHealthScore > 0) {
    failures.push(`Failed to fetch contract health score: ${healthRes.status} ${healthRes.statusText}`);
  }

  const passed = failures.length === 0;

  // Output formatting
  const outputs = {
    status: passed ? 'passed' : 'failed',
    'health-score': healthScore != null ? String(healthScore) : '',
    'min-ttl': String(minTtlFound),
    'storage-count': String(storageCount),
  };

  if (process.env.GITHUB_OUTPUT) {
    const outputLines = Object.entries(outputs)
      .map(([k, v]) => `${k}=${v}\n`)
      .join('');
    try {
      fs.appendFileSync(process.env.GITHUB_OUTPUT, outputLines);
    } catch {
      // ignore
    }
  }

  if (process.env.GITHUB_STEP_SUMMARY) {
    const summary = [
      `### Sorolens Contract Check: \`${contractId}\``,
      '',
      `| Metric | Value | Threshold | Result |`,
      `| --- | --- | --- | --- |`,
      `| Status | **${passed ? 'PASSED ✅' : 'FAILED ❌'}** | - | ${passed ? '✅' : '❌'} |`,
      `| Storage Entries | ${storageCount} | - | ℹ️ |`,
      `| Min Remaining TTL | ${minTtlFound} ledgers | ${minTtlLedgers > 0 ? minTtlLedgers + ' ledgers' : 'None'} | ${minTtlLedgers > 0 && minTtlFound < minTtlLedgers ? '❌' : '✅'} |`,
      `| Health Score | ${healthScore != null ? healthScore : 'N/A'} | ${minHealthScore > 0 ? minHealthScore : 'None'} | ${minHealthScore > 0 && (healthScore == null || healthScore < minHealthScore) ? '❌' : '✅'} |`,
      '',
      ...(failures.length > 0 ? ['#### Failures:', ...failures.map((f) => `- ❌ ${f}`)] : ['All assertions passed.']),
      '',
    ].join('\n');
    try {
      fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, summary);
    } catch {
      // ignore
    }
  }

  if (!passed) {
    for (const f of failures) {
      errorLog(`::error::${f}`);
    }
    const err = new Error(`sorolens-check failed with ${failures.length} threshold violation(s):\n${failures.join('\n')}`);
    err.failures = failures;
    throw err;
  }

  log(`[sorolens-check] ✅ All checks passed for contract ${contractId}`);
  return outputs;
}

const isEntry = process.argv[1] && import.meta.url.endsWith(process.argv[1].replace(/\\/g, '/'));
if (isEntry) {
  runCheck()
    .then(() => process.exit(0))
    .catch((err) => {
      console.error(`[sorolens-check] ❌ ${err.message}`);
      process.exit(1);
    });
}
