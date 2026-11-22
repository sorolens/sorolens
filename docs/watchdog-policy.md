# Watchdog monitoring policies

The watchdog contract can store one bounded monitoring policy per registered
contract so thresholds are auditable and portable between operators.

## Layout

Policies live in **persistent** storage under `DataKey::Policy(contract_id)`:

| Field                | Type    | Rule                                              |
| -------------------- | ------- | ------------------------------------------------- |
| `warning_threshold`  | u32     | `>= 1`, alerts per window that mark a contract degraded |
| `critical_threshold` | u32     | `>= warning_threshold`, `<= 1_000_000`            |
| `expected_interval`  | u64     | seconds between status reports, `1..=31_536_000`  |
| `owner`              | Address | operator the policy belongs to                    |

The struct is fixed-size, so oversized policies cannot be expressed. Invalid
values fail with `Error::InvalidPolicy`; batches must hold 1..=20 entries
(`Error::BatchSizeInvalid`).

## Entry points

- `set_policy(admin, contract_id, policy)`: admin only (`require_auth` plus an
  admin check). The contract must be registered. Emits `PolicyUpdated`.
- `set_policies_batch(admin, entries)`: atomic; one invalid entry aborts all.
- `get_policy(contract_id) -> Option<MonitoringPolicy>` and
  `get_policies_batch(contract_ids)`: read-only.

Every write and read extends the policy entry TTL (and the instance TTL) with
the contract's standard thresholds. `deregister_contract` removes the policy.

## Writing a policy (admin)

```bash
soroban contract invoke --id $WATCHDOG --network testnet \
  --source-account $ADMIN_SECRET -- set_policy --admin $ADMIN \
  --contract_id $MONITORED \
  --policy '{"warning_threshold":3,"critical_threshold":10,"expected_interval":300,"owner":"'$OWNER'"}'
```

Writes are intentionally not exposed by the Sorolens CLI or SDK.

## Reading a policy

```bash
sorolens watchdog policy get $MONITORED --watchdog-contract-id $WATCHDOG \
  --source-account $ADDRESS
```

The Rust SDK exposes `sorolens_sdk::policy::MonitoringPolicy` to parse the
JSON result and validate it against the same bounds.
