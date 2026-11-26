#![no_std]

//! Sorolens Watchdog contract.
//!
//! On-chain health tracking for other Soroban contracts. Contract owners
//! register a contract they operate, then periodically push health status
//! updates or alerts. The Sorolens indexer subscribes to the events emitted
//! here and materialises them into the dashboard.

use soroban_sdk::{
    contract, contracterror, contractevent, contractimpl, contracttype, panic_with_error, Address,
    Env, String, Symbol, Vec,
};

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------


/// A monitoring policy was stored or replaced by the admin.
#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PolicyUpdated {
    #[topic]
    pub contract_id: Address,
    pub policy: MonitoringPolicy,
    pub timestamp: u64,
}
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum HealthStatus {
    Healthy,
    Degraded,
    Unresponsive,
    Custom(String),
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum AlertSeverity {
    Info,
    Warning,
    Critical,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ContractHealth {
    pub contract_id: Address,
    pub name: Symbol,
    pub owner: Address,
    pub status: HealthStatus,
    pub last_check: u64,
    pub check_interval: u64,
    pub registered_at: u64,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Alert {
    pub contract_id: Address,
    pub severity: AlertSeverity,
    pub message: String,
    pub timestamp: u64,
}

/// A single contract to register in a batch call.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Registration {
    pub contract_id: Address,
    pub name: Symbol,
    pub check_interval: u64,
}

/// On-chain monitoring policy for one monitored contract (issue #813).
///
/// Fixed-size on purpose: three integers and an address, so storage cost is
/// constant and an oversized policy cannot be expressed.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct MonitoringPolicy {
    /// Alerts per monitoring window at which the contract is considered
    /// degraded (warning severity). Must be at least 1.
    pub warning_threshold: u32,
    /// Alerts per monitoring window at which the contract is considered
    /// critical. Must be >= `warning_threshold`.
    pub critical_threshold: u32,
    /// Expected seconds between status reports. 1..=`MAX_POLICY_INTERVAL`.
    pub expected_interval: u64,
    /// Operator the policy belongs to (informational; writes are admin-only).
    pub owner: Address,
}

/// A policy to store for one contract in a batch call.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PolicyEntry {
    pub contract_id: Address,
    pub policy: MonitoringPolicy,
}

/// Outcome of a single registration within a batch.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum BatchResult {
    /// The contract was registered successfully.
    Success,
    /// The contract was skipped with a reason (duplicate, invalid, etc.).
    Skipped(String),
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AdminSet {
    pub admins: Vec<Address>,
    pub threshold: u32,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProposalType {
    Pause,
    Unpause,
    SetAdmin(AdminSet),
    Upgrade(soroban_sdk::BytesN<32>),
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Proposal {
    pub id: u64,
    pub proposer: Address,
    pub proposal_type: ProposalType,
    pub approvals: Vec<Address>,
    pub expiry: u64,
    pub executed: bool,
    pub cancelled: bool,
}

#[contracttype]
#[derive(Clone)]
enum DataKey {
    Admin,
    Registry,        // Vec<Address>: list of monitored contract ids
    Health(Address), // ContractHealth by contract id
    Alerts(Address), // Vec<Alert> by contract id
    Paused,          // bool: emergency stop flag; absent means not paused
    PendingAdmin,    // Address: nominated next admin awaiting acceptance
    Policy(Address), // MonitoringPolicy by contract id (persistent)
    AdminSetKey,     // AdminSet: M-of-N admin set if active
    Proposal(u64),   // Proposal by proposal id
    ProposalCount,   // u64: counter for proposal IDs
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

#[contracterror]
#[derive(Copy, Clone, Debug, Eq, PartialEq, PartialOrd, Ord)]
#[repr(u32)]
pub enum Error {
    /// The admin has paused the contract; state-changing calls are rejected.
    ContractPaused = 1,
    /// The policy failed validation (zero or inverted thresholds, or an
    /// interval outside the allowed range).
    InvalidPolicy = 2,
    /// A policy batch was empty or larger than `MAX_BATCH_SIZE`.
    BatchSizeInvalid = 3,
    /// Not an admin or authorized proposal signer.
    NotAdmin = 4,
    /// Proposal not found or already processed/cancelled.
    ProposalNotFound = 5,
    /// Proposal has expired.
    ProposalExpired = 6,
    /// Already approved by this admin.
    AlreadyApproved = 7,
    /// Invalid admin set or threshold parameters.
    InvalidAdminSet = 8,
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ContractRegistered {
    #[topic]
    pub contract_id: Address,
    pub name: Symbol,
    pub owner: Address,
    pub timestamp: u64,
}

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ContractDeregistered {
    #[topic]
    pub contract_id: Address,
    pub timestamp: u64,
}

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HealthCheckEvent {
    #[topic]
    pub contract_id: Address,
    pub status: HealthStatus,
    pub timestamp: u64,
    pub metadata: String,
}

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ContractAlert {
    #[topic]
    pub contract_id: Address,
    #[topic]
    pub severity: AlertSeverity,
    pub message: String,
    pub timestamp: u64,
}

/// An admin hand-off has started: `current_admin` nominated `new_admin`.
#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AdminTransferProposed {
    #[topic]
    pub current_admin: Address,
    pub new_admin: Address,
}

/// The nominated address accepted and is now the contract admin.
#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AdminTransferAccepted {
    #[topic]
    pub new_admin: Address,
}

/// A pending admin nomination was withdrawn; `current_admin` keeps the role.
#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AdminTransferCancelled {
    #[topic]
    pub current_admin: Address,
    pub pending_admin: Address,
}

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProposalCreated {
    #[topic]
    pub proposal_id: u64,
    pub proposer: Address,
}

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProposalApproved {
    #[topic]
    pub proposal_id: u64,
    #[topic]
    pub approver: Address,
}

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProposalExecuted {
    #[topic]
    pub proposal_id: u64,
}

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProposalCancelled {
    #[topic]
    pub proposal_id: u64,
}

// ---------------------------------------------------------------------------
// Contract
// ---------------------------------------------------------------------------

#[contract]
pub struct WatchdogContract;

const MAX_ALERTS_PER_CONTRACT: u32 = 100;
const MAX_BATCH_SIZE: u32 = 20;
/// Upper bounds for policy values (one year in seconds; 1M alerts per window).
const MAX_POLICY_INTERVAL: u64 = 31_536_000;
const MAX_POLICY_THRESHOLD: u32 = 1_000_000;

// TTL management (issue #573). Every entry this contract writes is durable
// state: losing the admin, registry, health record or alert history means the
// watchdog can no longer be operated or audited. Ledger TTLs on Stellar are
// finite, so reads and writes bump the entries they touch back to
// `TTL_EXTEND_TO` whenever the remaining lifetime falls below `TTL_THRESHOLD`.
//
// At ~5 second ledgers: 17,280 ledgers is ~1 day and 518,400 is ~30 days, so a
// contract that is read at least once a day never lets its state archive.
const TTL_THRESHOLD: u32 = 17_280;
const TTL_EXTEND_TO: u32 = 518_400;

#[contractimpl]
impl WatchdogContract {
    /// Initialise the watchdog with a single administrator. Must be called
    /// exactly once immediately after deployment.
    pub fn initialize(env: Env, admin: Address) {
        if env.storage().instance().has(&DataKey::Admin) {
            panic!("already initialized");
        }
        admin.require_auth();
        env.storage().instance().set(&DataKey::Admin, &admin);
        env.storage()
            .instance()
            .set(&DataKey::Registry, &Vec::<Address>::new(&env));
        Self::bump_instance_ttl(&env);
    }

    /// Register a contract for monitoring. Only the admin, or the account
    /// that will own the record, can register it.
    pub fn register_contract(
        env: Env,
        caller: Address,
        contract_id: Address,
        name: Symbol,
        check_interval: u64,
    ) {
        Self::ensure_not_paused(&env);
        caller.require_auth();
        Self::require_admin_or(&env, &caller);

        let health_key = DataKey::Health(contract_id.clone());
        if env.storage().persistent().has(&health_key) {
            panic!("contract already registered");
        }

        Self::store_registration(
            &env,
            &caller,
            &Registration {
                contract_id,
                name,
                check_interval,
            },
        );
    }

    /// Register up to {@link MAX_BATCH_SIZE} contracts in a single call.
    ///
    /// Each registration is applied independently: a duplicate (or otherwise
    /// invalid) entry is skipped without reverting the whole batch, and a
    /// `ContractRegistered` event is emitted for every successful one. The
    /// caller receives a `Vec<BatchResult>` describing what happened per
    /// entry.
    ///
    /// # Errors
    ///
    /// Panics with `"batch size exceeds maximum"` if `registrations.len()`
    /// exceeds {@link MAX_BATCH_SIZE}, before any entry is processed. An
    /// empty batch returns an empty `Vec` without panicking.
    pub fn register_contracts_batch(
        env: Env,
        caller: Address,
        registrations: Vec<Registration>,
    ) -> Vec<BatchResult> {
        Self::ensure_not_paused(&env);
        caller.require_auth();
        Self::require_admin_or(&env, &caller);

        if registrations.len() > MAX_BATCH_SIZE {
            panic!("batch size exceeds maximum");
        }

        let mut results: Vec<BatchResult> = Vec::new(&env);
        for registration in registrations.iter() {
            if registration.check_interval == 0 {
                results.push_back(BatchResult::Skipped(String::from_str(
                    &env,
                    "check interval must be greater than zero",
                )));
                continue;
            }
            let health_key = DataKey::Health(registration.contract_id.clone());
            if env.storage().persistent().has(&health_key) {
                results.push_back(BatchResult::Skipped(String::from_str(
                    &env,
                    "contract already registered",
                )));
                continue;
            }
            Self::store_registration(&env, &caller, &registration);
            results.push_back(BatchResult::Success);
        }
        results
    }
    /// Deregister a contract. Only the owner (recorded at registration) or
    /// the admin can deregister.
    pub fn deregister_contract(env: Env, caller: Address, contract_id: Address) {
        Self::ensure_not_paused(&env);
        caller.require_auth();

        let health_key = DataKey::Health(contract_id.clone());
        let record: ContractHealth = env
            .storage()
            .persistent()
            .get(&health_key)
            .unwrap_or_else(|| panic!("contract not registered"));

        if caller != record.owner {
            Self::require_admin_only(&env, &caller);
        }

        env.storage().persistent().remove(&health_key);
        env.storage()
            .persistent()
            .remove(&DataKey::Alerts(contract_id.clone()));
        env.storage()
            .persistent()
            .remove(&DataKey::Policy(contract_id.clone()));

        let registry: Vec<Address> = env
            .storage()
            .instance()
            .get(&DataKey::Registry)
            .unwrap_or_else(|| Vec::new(&env));
        let mut updated: Vec<Address> = Vec::new(&env);
        for entry in registry.iter() {
            if entry != contract_id {
                updated.push_back(entry);
            }
        }
        env.storage().instance().set(&DataKey::Registry, &updated);
        Self::bump_instance_ttl(&env);

        ContractDeregistered {
            contract_id,
            timestamp: env.ledger().timestamp(),
        }
        .publish(&env);
    }

    /// Push a new health status for a monitored contract. Only the recorded
    /// owner or the admin may push status updates for that contract.
    pub fn report_status(
        env: Env,
        caller: Address,
        contract_id: Address,
        status: HealthStatus,
        metadata: String,
    ) {
        Self::ensure_not_paused(&env);
        caller.require_auth();

        let health_key = DataKey::Health(contract_id.clone());
        let mut record: ContractHealth = env
            .storage()
            .persistent()
            .get(&health_key)
            .unwrap_or_else(|| panic!("contract not registered"));

        if caller != record.owner {
            Self::require_admin_only(&env, &caller);
        }

        let now = env.ledger().timestamp();
        record.status = status.clone();
        record.last_check = now;
        env.storage().persistent().set(&health_key, &record);
        Self::bump_persistent_ttl(&env, &health_key);
        Self::bump_instance_ttl(&env);

        HealthCheckEvent {
            contract_id,
            status,
            timestamp: now,
            metadata,
        }
        .publish(&env);
    }

    /// Emit an alert against a monitored contract. Auth model matches
    /// `report_status`.
    pub fn report_alert(
        env: Env,
        caller: Address,
        contract_id: Address,
        severity: AlertSeverity,
        message: String,
    ) {
        Self::ensure_not_paused(&env);
        caller.require_auth();

        let health_key = DataKey::Health(contract_id.clone());
        let record: ContractHealth = env
            .storage()
            .persistent()
            .get(&health_key)
            .unwrap_or_else(|| panic!("contract not registered"));

        if caller != record.owner {
            Self::require_admin_only(&env, &caller);
        }

        let now = env.ledger().timestamp();
        let alert = Alert {
            contract_id: contract_id.clone(),
            severity: severity.clone(),
            message: message.clone(),
            timestamp: now,
        };

        let alerts_key = DataKey::Alerts(contract_id.clone());
        let mut alerts: Vec<Alert> = env
            .storage()
            .persistent()
            .get(&alerts_key)
            .unwrap_or_else(|| Vec::new(&env));
        alerts.push_back(alert);
        while alerts.len() > MAX_ALERTS_PER_CONTRACT {
            alerts.pop_front();
        }
        env.storage().persistent().set(&alerts_key, &alerts);
        Self::bump_persistent_ttl(&env, &alerts_key);
        Self::bump_persistent_ttl(&env, &health_key);
        Self::bump_instance_ttl(&env);

        ContractAlert {
            contract_id,
            severity,
            message,
            timestamp: now,
        }
        .publish(&env);
    }

    // ---- admin: emergency stop ---------------------------------------------

    /// Pause the contract (kill switch). While paused, every state-changing
    /// method except `pause`/`unpause` fails with `Error::ContractPaused`;
    /// read-only queries keep working. Admin only. Pausing an already
    /// paused contract is a no-op.
    pub fn pause(env: Env, admin: Address) {
        admin.require_auth();
        Self::require_admin_only(&env, &admin);
        env.storage().instance().set(&DataKey::Paused, &true);
        Self::bump_instance_ttl(&env);
    }

    /// Lift a pause set by `pause`. Admin only. Unpausing a contract that is
    /// not paused is a no-op.
    pub fn unpause(env: Env, admin: Address) {
        admin.require_auth();
        Self::require_admin_only(&env, &admin);
        env.storage().instance().remove(&DataKey::Paused);
        Self::bump_instance_ttl(&env);
    }

    // ---- admin: two-step transfer ------------------------------------------

    /// Start a two-step admin hand-off by nominating `new_admin`. Admin only.
    ///
    /// The nomination is stored as a *pending* admin and does not take effect
    /// until that exact address calls `accept_admin`. A typo or an
    /// unavailable nominee therefore cannot lock the contract out of its
    /// admin role: the current admin stays in control until acceptance.
    /// Nominating again replaces any earlier pending nomination.
    ///
    /// This call is intentionally **not** gated by `ensure_not_paused`:
    /// rotating admin keys (for example while the kill switch is engaged)
    /// must remain possible for the current admin, like `pause`/`unpause`.
    pub fn propose_admin(env: Env, current_admin: Address, new_admin: Address) {
        current_admin.require_auth();
        Self::require_admin_only(&env, &current_admin);

        if new_admin == current_admin {
            panic!("new admin is already the current admin");
        }

        env.storage()
            .instance()
            .set(&DataKey::PendingAdmin, &new_admin);
        Self::bump_instance_ttl(&env);

        AdminTransferProposed {
            current_admin,
            new_admin,
        }
        .publish(&env);
    }

    /// Complete a pending hand-off. Only the nominated address may call this,
    /// and it must authorize the call.
    ///
    /// On success the nominee becomes the stored admin and the pending slot
    /// is cleared. Nothing is written if there is no live nomination or if
    /// the caller is not the nominee.
    pub fn accept_admin(env: Env, new_admin: Address) {
        new_admin.require_auth();

        let pending: Address = env
            .storage()
            .instance()
            .get(&DataKey::PendingAdmin)
            .unwrap_or_else(|| panic!("no admin transfer proposed"));

        if new_admin != pending {
            panic!("caller is not the pending admin");
        }

        env.storage().instance().set(&DataKey::Admin, &pending);
        env.storage().instance().remove(&DataKey::PendingAdmin);
        Self::bump_instance_ttl(&env);

        AdminTransferAccepted { new_admin }.publish(&env);
    }

    /// Withdraw the pending nomination, leaving the current admin in place.
    /// Admin only; panics if there is nothing pending.
    pub fn cancel_admin_proposal(env: Env, current_admin: Address) {
        current_admin.require_auth();
        Self::require_admin_only(&env, &current_admin);

        let pending: Address = env
            .storage()
            .instance()
            .get(&DataKey::PendingAdmin)
            .unwrap_or_else(|| panic!("no admin transfer proposed"));

        env.storage().instance().remove(&DataKey::PendingAdmin);
        Self::bump_instance_ttl(&env);

        AdminTransferCancelled {
            current_admin,
            pending_admin: pending,
        }
        .publish(&env);
    }

    // ---- monitoring policies (issue #813) ----------------------------------

    /// Store or replace the monitoring policy of a registered contract.
    /// Admin only; the policy is validated and its TTL extended.
    pub fn set_policy(env: Env, admin: Address, contract_id: Address, policy: MonitoringPolicy) {
        Self::ensure_not_paused(&env);
        admin.require_auth();
        Self::require_admin_only(&env, &admin);
        Self::store_policy(&env, &contract_id, &policy);
        Self::bump_instance_ttl(&env);
    }

    /// Store policies for up to `MAX_BATCH_SIZE` contracts atomically: one
    /// invalid entry aborts the whole call.
    pub fn set_policies_batch(env: Env, admin: Address, entries: Vec<PolicyEntry>) {
        Self::ensure_not_paused(&env);
        admin.require_auth();
        Self::require_admin_only(&env, &admin);
        if entries.is_empty() || entries.len() > MAX_BATCH_SIZE {
            panic_with_error!(&env, Error::BatchSizeInvalid);
        }
        for entry in entries.iter() {
            Self::store_policy(&env, &entry.contract_id, &entry.policy);
        }
        Self::bump_instance_ttl(&env);
    }

    /// Read the policy of a contract, or `None` when none was set.
    pub fn get_policy(env: Env, contract_id: Address) -> Option<MonitoringPolicy> {
        let key = DataKey::Policy(contract_id);
        let policy = env.storage().persistent().get(&key);
        Self::bump_persistent_ttl(&env, &key);
        Self::bump_instance_ttl(&env);
        policy
    }

    /// Read policies for several contracts (at most `MAX_BATCH_SIZE`), in
    /// request order; `None` for contracts without a policy.
    pub fn get_policies_batch(
        env: Env,
        contract_ids: Vec<Address>,
    ) -> Vec<Option<MonitoringPolicy>> {
        if contract_ids.len() > MAX_BATCH_SIZE {
            panic_with_error!(&env, Error::BatchSizeInvalid);
        }
        let mut out: Vec<Option<MonitoringPolicy>> = Vec::new(&env);
        for id in contract_ids.iter() {
            out.push_back(Self::get_policy(env.clone(), id));
        }
        out
    }

    // ---- read-only queries -------------------------------------------------

    pub fn get_status(env: Env, contract_id: Address) -> ContractHealth {
        let key = DataKey::Health(contract_id);
        let record = env
            .storage()
            .persistent()
            .get(&key)
            .unwrap_or_else(|| panic!("contract not registered"));
        Self::bump_persistent_ttl(&env, &key);
        Self::bump_instance_ttl(&env);
        record
    }

    /// Gets a page of monitored contracts.
    pub fn get_monitored_page(env: Env, start: u32, limit: u32) -> (Vec<ContractHealth>, u32) {
        Self::bump_instance_ttl(&env);
        let registry: Vec<Address> = env
            .storage()
            .instance()
            .get(&DataKey::Registry)
            .unwrap_or_else(|| Vec::new(&env));
        let total = registry.len();
        let mut page = Vec::new(&env);
        if start < total {
            for i in start..core::cmp::min(start + limit, total) {
                if let Some(id) = registry.get(i) {
                    let key = DataKey::Health(id.clone());
                    if let Some(record) = env.storage().persistent().get::<_, ContractHealth>(&key)
                    {
                        Self::bump_persistent_ttl(&env, &key);
                        page.push_back(record);
                    }
                }
            }
        }
        (page, total)
    }

    pub fn get_all_monitored(env: Env) -> Vec<ContractHealth> {
        Self::bump_instance_ttl(&env);
        let registry: Vec<Address> = env
            .storage()
            .instance()
            .get(&DataKey::Registry)
            .unwrap_or_else(|| Vec::new(&env));
        let mut out: Vec<ContractHealth> = Vec::new(&env);
        for id in registry.iter() {
            let key = DataKey::Health(id);
            if let Some(record) = env.storage().persistent().get(&key) {
                Self::bump_persistent_ttl(&env, &key);
                out.push_back(record);
            }
        }
        out
    }

    pub fn get_alerts(env: Env, contract_id: Address) -> Vec<Alert> {
        Self::bump_instance_ttl(&env);
        let key = DataKey::Alerts(contract_id);
        let alerts = env
            .storage()
            .persistent()
            .get(&key)
            .unwrap_or_else(|| Vec::new(&env));
        Self::bump_persistent_ttl(&env, &key);
        alerts
    }

    /// Returns only the alerts for `contract_id` whose timestamp is strictly
    /// greater than `since_timestamp`, oldest first. Passing `0` returns the
    /// full history (same as `get_alerts`). The indexer passes the timestamp
    /// of the last alert it processed so each poll only reads new alerts.
    ///
    /// Alerts are appended in ledger-timestamp order, so the scan walks back
    /// from the newest alert and stops at the first one that is not newer.
    pub fn get_alerts_since(env: Env, contract_id: Address, since_timestamp: u64) -> Vec<Alert> {
        Self::bump_instance_ttl(&env);
        let key = DataKey::Alerts(contract_id);
        let alerts: Vec<Alert> = env
            .storage()
            .persistent()
            .get(&key)
            .unwrap_or_else(|| Vec::new(&env));
        Self::bump_persistent_ttl(&env, &key);

        let mut first_new = alerts.len();
        while first_new > 0 {
            match alerts.get(first_new - 1) {
                Some(alert) if alert.timestamp > since_timestamp => first_new -= 1,
                _ => break,
            }
        }
        alerts.slice(first_new..)
    }

    pub fn get_monitored_count(env: Env) -> u32 {
        Self::bump_instance_ttl(&env);
        let registry: Vec<Address> = env
            .storage()
            .instance()
            .get(&DataKey::Registry)
            .unwrap_or_else(|| Vec::new(&env));
        registry.len()
    }

    pub fn admin(env: Env) -> Address {
        Self::bump_instance_ttl(&env);
        env.storage()
            .instance()
            .get(&DataKey::Admin)
            .unwrap_or_else(|| panic!("not initialized"))
    }

    pub fn admin_set(env: Env) -> Option<AdminSet> {
        Self::bump_instance_ttl(&env);
        env.storage()
            .instance()
            .get::<_, AdminSet>(&DataKey::AdminSetKey)
    }

    pub fn set_admin_set(env: Env, caller: Address, admins: Vec<Address>, threshold: u32) -> u64 {
        caller.require_auth();
        Self::require_admin_or_member(&env, &caller);
        if threshold == 0 || threshold > admins.len() || admins.is_empty() {
            panic_with_error!(&env, Error::InvalidAdminSet);
        }
        let admin_set = AdminSet { admins, threshold };
        Self::create_proposal_internal(env, caller, ProposalType::SetAdmin(admin_set))
    }

    pub fn propose_pause(env: Env, caller: Address) -> u64 {
        caller.require_auth();
        Self::require_admin_or_member(&env, &caller);
        Self::create_proposal_internal(env, caller, ProposalType::Pause)
    }

    pub fn propose_unpause(env: Env, caller: Address) -> u64 {
        caller.require_auth();
        Self::require_admin_or_member(&env, &caller);
        Self::create_proposal_internal(env, caller, ProposalType::Unpause)
    }

    pub fn approve_proposal(env: Env, caller: Address, proposal_id: u64) {
        caller.require_auth();
        Self::require_admin_or_member(&env, &caller);

        let key = DataKey::Proposal(proposal_id);
        let mut proposal: Proposal = env.storage().persistent().get(&key).unwrap_or_else(|| panic_with_error!(&env, Error::ProposalNotFound));
        if proposal.executed || proposal.cancelled {
            panic_with_error!(&env, Error::ProposalNotFound);
        }
        let now = env.ledger().timestamp();
        if proposal.expiry < now {
            panic_with_error!(&env, Error::ProposalExpired);
        }
        for a in proposal.approvals.iter() {
            if a == caller {
                panic_with_error!(&env, Error::AlreadyApproved);
            }
        }
        proposal.approvals.push_back(caller.clone());
        env.storage().persistent().set(&key, &proposal);
        Self::bump_persistent_ttl(&env, &key);
        Self::bump_instance_ttl(&env);

        ProposalApproved {
            proposal_id,
            approver: caller,
        }
        .publish(&env);

        let threshold = Self::current_threshold(&env);
        if proposal.approvals.len() >= threshold {
            Self::execute_proposal_internal(env, proposal_id, &mut proposal);
        }
    }

    pub fn execute_proposal(env: Env, caller: Address, proposal_id: u64) {
        caller.require_auth();
        Self::require_admin_or_member(&env, &caller);

        let key = DataKey::Proposal(proposal_id);
        let mut proposal: Proposal = env.storage().persistent().get(&key).unwrap_or_else(|| panic_with_error!(&env, Error::ProposalNotFound));
        if proposal.executed || proposal.cancelled {
            panic_with_error!(&env, Error::ProposalNotFound);
        }
        let now = env.ledger().timestamp();
        if proposal.expiry < now {
            panic_with_error!(&env, Error::ProposalExpired);
        }
        let threshold = Self::current_threshold(&env);
        if proposal.approvals.len() < threshold {
            panic!("threshold not met");
        }
        Self::execute_proposal_internal(env, proposal_id, &mut proposal);
    }

    pub fn cancel_proposal(env: Env, caller: Address, proposal_id: u64) {
        caller.require_auth();
        Self::require_admin_or_member(&env, &caller);

        let key = DataKey::Proposal(proposal_id);
        let mut proposal: Proposal = env.storage().persistent().get(&key).unwrap_or_else(|| panic_with_error!(&env, Error::ProposalNotFound));
        if proposal.executed || proposal.cancelled {
            panic_with_error!(&env, Error::ProposalNotFound);
        }
        if proposal.proposer != caller {
            Self::require_admin_only(&env, &caller);
        }
        proposal.cancelled = true;
        env.storage().persistent().set(&key, &proposal);
        Self::bump_persistent_ttl(&env, &key);
        Self::bump_instance_ttl(&env);

        ProposalCancelled { proposal_id }.publish(&env);
    }

    pub fn get_proposal(env: Env, proposal_id: u64) -> Proposal {
        Self::bump_instance_ttl(&env);
        let key = DataKey::Proposal(proposal_id);
        let proposal: Proposal = env.storage().persistent().get(&key).unwrap_or_else(|| panic_with_error!(&env, Error::ProposalNotFound));
        Self::bump_persistent_ttl(&env, &key);
        proposal
    }

    pub fn is_paused(env: Env) -> bool {
        Self::bump_instance_ttl(&env);
        env.storage()
            .instance()
            .get(&DataKey::Paused)
            .unwrap_or(false)
    }

    /// The address currently nominated to become admin, if a hand-off is in
    /// progress.
    pub fn pending_admin(env: Env) -> Option<Address> {
        Self::bump_instance_ttl(&env);
        env.storage()
            .instance()
            .get::<_, Address>(&DataKey::PendingAdmin)
    }

    // ---- helpers -----------------------------------------------------------

    /// Top up the contract instance's TTL (Admin, Registry, Paused and
    /// PendingAdmin all live in instance storage).
    fn bump_instance_ttl(env: &Env) {
        env.storage()
            .instance()
            .extend_ttl(TTL_THRESHOLD, TTL_EXTEND_TO);
    }

    /// Top up a persistent entry's TTL. Missing keys are skipped: the host
    /// rejects `extend_ttl` for an entry that does not exist, and read paths
    /// such as `get_alerts_since` serve unknown contracts with an empty list.
    fn bump_persistent_ttl(env: &Env, key: &DataKey) {
        if env.storage().persistent().has(key) {
            env.storage()
                .persistent()
                .extend_ttl(key, TTL_THRESHOLD, TTL_EXTEND_TO);
        }
    }

    fn store_policy(env: &Env, contract_id: &Address, policy: &MonitoringPolicy) {
        if policy.warning_threshold == 0
            || policy.warning_threshold > policy.critical_threshold
            || policy.critical_threshold > MAX_POLICY_THRESHOLD
            || policy.expected_interval == 0
            || policy.expected_interval > MAX_POLICY_INTERVAL
        {
            panic_with_error!(env, Error::InvalidPolicy);
        }
        if !env
            .storage()
            .persistent()
            .has(&DataKey::Health(contract_id.clone()))
        {
            panic!("contract not registered");
        }
        let key = DataKey::Policy(contract_id.clone());
        env.storage().persistent().set(&key, policy);
        Self::bump_persistent_ttl(env, &key);
        PolicyUpdated {
            contract_id: contract_id.clone(),
            policy: policy.clone(),
            timestamp: env.ledger().timestamp(),
        }
        .publish(env);
    }

    fn ensure_not_paused(env: &Env) {
        if Self::is_paused(env.clone()) {
            panic_with_error!(env, Error::ContractPaused);
        }
    }

    fn store_registration(env: &Env, caller: &Address, registration: &Registration) {
        let now = env.ledger().timestamp();
        let record = ContractHealth {
            contract_id: registration.contract_id.clone(),
            name: registration.name.clone(),
            owner: caller.clone(),
            status: HealthStatus::Healthy,
            last_check: now,
            check_interval: registration.check_interval,
            registered_at: now,
        };
        env.storage()
            .persistent()
            .set(&DataKey::Health(registration.contract_id.clone()), &record);
        env.storage().persistent().set(
            &DataKey::Alerts(registration.contract_id.clone()),
            &Vec::<Alert>::new(env),
        );

        let mut registry: Vec<Address> = env
            .storage()
            .instance()
            .get(&DataKey::Registry)
            .unwrap_or_else(|| Vec::new(env));
        registry.push_back(registration.contract_id.clone());
        env.storage().instance().set(&DataKey::Registry, &registry);

        Self::bump_instance_ttl(env);
        Self::bump_persistent_ttl(env, &DataKey::Health(registration.contract_id.clone()));
        Self::bump_persistent_ttl(env, &DataKey::Alerts(registration.contract_id.clone()));

        ContractRegistered {
            contract_id: registration.contract_id.clone(),
            name: registration.name.clone(),
            owner: caller.clone(),
            timestamp: now,
        }
        .publish(env);
    }

    fn require_admin_or(env: &Env, caller: &Address) {
        // Any authenticated caller can self-register a contract they own;
        // this hook exists so that in future we can gate registration
        // behind admin-only enrolment without touching call sites.
        let _ = env
            .storage()
            .instance()
            .get::<_, Address>(&DataKey::Admin)
            .unwrap_or_else(|| panic!("not initialized"));
        let _ = caller;
    }

    fn require_admin_only(env: &Env, caller: &Address) {
        if let Some(admin_set) = env.storage().instance().get::<_, AdminSet>(&DataKey::AdminSetKey) {
            let mut found = false;
            for a in admin_set.admins.iter() {
                if a == *caller {
                    found = true;
                    break;
                }
            }
            if !found {
                panic!("only admin or owner may perform this action");
            }
        } else {
        let admin: Address = env
            .storage()
            .instance()
            .get(&DataKey::Admin)
            .unwrap_or_else(|| panic!("not initialized"));
        if caller != &admin {
            panic!("only admin or owner may perform this action");
        }
    }
}

    fn require_admin_or_member(env: &Env, caller: &Address) {
        if let Some(admin_set) = env.storage().instance().get::<_, AdminSet>(&DataKey::AdminSetKey) {
            let mut found = false;
            for a in admin_set.admins.iter() {
                if a == *caller {
                    found = true;
                    break;
                }
            }
            if !found {
                panic_with_error!(env, Error::NotAdmin);
            }
        } else {
            let admin: Address = env
                .storage()
                .instance()
                .get(&DataKey::Admin)
                .unwrap_or_else(|| panic!("not initialized"));
            if caller != &admin {
                panic_with_error!(env, Error::NotAdmin);
            }
        }
    }

    fn current_threshold(env: &Env) -> u32 {
        if let Some(admin_set) = env.storage().instance().get::<_, AdminSet>(&DataKey::AdminSetKey) {
            admin_set.threshold
        } else {
            1
        }
    }

    fn create_proposal_internal(env: Env, proposer: Address, proposal_type: ProposalType) -> u64 {
        let proposal_count: u64 = env.storage().instance().get(&DataKey::ProposalCount).unwrap_or(0);
        let proposal_id = proposal_count + 1;
        env.storage().instance().set(&DataKey::ProposalCount, &proposal_id);

        let now = env.ledger().timestamp();
        let expiry = now + 604800; // 7 days expiry

        let mut approvals = Vec::new(&env);
        approvals.push_back(proposer.clone());

        let proposal = Proposal {
            id: proposal_id,
            proposer: proposer.clone(),
            proposal_type,
            approvals,
            expiry,
            executed: false,
            cancelled: false,
        };

        let key = DataKey::Proposal(proposal_id);
        env.storage().persistent().set(&key, &proposal);
        Self::bump_persistent_ttl(&env, &key);
        Self::bump_instance_ttl(&env);

        ProposalCreated {
            proposal_id,
            proposer,
        }
        .publish(&env);

        let threshold = Self::current_threshold(&env);
        if proposal.approvals.len() >= threshold {
            let mut p = proposal;
            Self::execute_proposal_internal(env, proposal_id, &mut p);
        }

        proposal_id
    }

    fn execute_proposal_internal(env: Env, proposal_id: u64, proposal: &mut Proposal) {
        proposal.executed = true;
        let key = DataKey::Proposal(proposal_id);
        env.storage().persistent().set(&key, proposal);
        Self::bump_persistent_ttl(&env, &key);
        Self::bump_instance_ttl(&env);

        match &proposal.proposal_type {
            ProposalType::Pause => {
                env.storage().instance().set(&DataKey::Paused, &true);
            }
            ProposalType::Unpause => {
                env.storage().instance().remove(&DataKey::Paused);
            }
            ProposalType::SetAdmin(admin_set) => {
                env.storage().instance().set(&DataKey::AdminSetKey, admin_set);
                if !admin_set.admins.is_empty() {
                    env.storage().instance().set(&DataKey::Admin, &admin_set.admins.get(0).unwrap());
                }
            }
            ProposalType::Upgrade(wasm_hash) => {
                env.deployer().update_current_contract_wasm(wasm_hash.clone());
            }
        }

        ProposalExecuted { proposal_id }.publish(&env);
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod test {
    extern crate std;

    use super::*;
    use soroban_sdk::{
        symbol_short,
        testutils::{Address as _, AuthorizedFunction, AuthorizedInvocation, Ledger},
        Env, IntoVal, String as SString,
    };

    fn setup(env: &Env) -> (Address, WatchdogContractClient<'_>) {
        let contract_id = env.register(WatchdogContract {}, ());
        let client = WatchdogContractClient::new(env, &contract_id);
        let admin = Address::generate(env);
        env.mock_all_auths();
        client.initialize(&admin);
        (admin, client)
    }

    fn set_timestamp(env: &Env, ts: u64) {
        env.ledger().with_mut(|li| {
            li.timestamp = ts;
        });
    }

    #[test]
    fn initialize_stores_admin_and_empty_registry() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        assert_eq!(client.admin(), admin);
        assert_eq!(client.get_monitored_count(), 0);
        assert_eq!(client.get_all_monitored().len(), 0);
    }

    #[test]
    #[should_panic(expected = "already initialized")]
    fn initialize_twice_panics() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let other = Address::generate(&env);
        client.initialize(&other);
    }

    #[test]
    fn register_and_query_flow() {
        let env = Env::default();
        set_timestamp(&env, 1_700_000_000);
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);

        client.register_contract(&owner, &monitored, &symbol_short!("app_v1"), &300u64);

        assert_eq!(client.get_monitored_count(), 1);
        let record = client.get_status(&monitored);
        assert_eq!(record.contract_id, monitored);
        assert_eq!(record.owner, owner);
        assert_eq!(record.name, symbol_short!("app_v1"));
        assert_eq!(record.status, HealthStatus::Healthy);
        assert_eq!(record.check_interval, 300);
        assert_eq!(record.registered_at, 1_700_000_000);
    }

    #[test]
    #[should_panic(expected = "contract already registered")]
    fn duplicate_registration_panics() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("app"), &60u64);
        client.register_contract(&owner, &monitored, &symbol_short!("app"), &60u64);
    }

    #[test]
    fn batch_registering_multiple_contracts_succeeds() {
        let env = Env::default();
        set_timestamp(&env, 1_700_000_000);
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);

        let mut registrations: Vec<Registration> = Vec::new(&env);
        for _ in 0u32..5u32 {
            let id = Address::generate(&env);
            registrations.push_back(Registration {
                contract_id: id,
                name: symbol_short!("svc"),
                check_interval: 60,
            });
        }

        let results = client.register_contracts_batch(&owner, &registrations);

        assert_eq!(results.len(), 5);
        for idx in 0u32..5u32 {
            assert_eq!(results.get(idx).unwrap(), BatchResult::Success);
        }
        assert_eq!(client.get_monitored_count(), 5);
        assert_eq!(client.get_all_monitored().len(), 5);
    }

    #[test]
    fn batch_skips_duplicates_but_registers_the_rest() {
        let env = Env::default();
        set_timestamp(&env, 1_700_000_000);
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let existing = Address::generate(&env);
        client.register_contract(&owner, &existing, &symbol_short!("old"), &60u64);

        let fresh = Address::generate(&env);
        let mut registrations: Vec<Registration> = Vec::new(&env);
        registrations.push_back(Registration {
            contract_id: existing.clone(),
            name: symbol_short!("dup"),
            check_interval: 60,
        });
        registrations.push_back(Registration {
            contract_id: fresh.clone(),
            name: symbol_short!("new"),
            check_interval: 120,
        });

        let results = client.register_contracts_batch(&owner, &registrations);

        assert_eq!(results.len(), 2);
        assert_eq!(
            results.get(0).unwrap(),
            BatchResult::Skipped(SString::from_str(&env, "contract already registered",))
        );
        assert_eq!(results.get(1).unwrap(), BatchResult::Success);
        assert_eq!(client.get_monitored_count(), 2);
        assert!(client.get_status(&fresh).check_interval == 120);
    }

    #[test]
    #[should_panic(expected = "batch size exceeds maximum")]
    fn batch_over_max_size_errors_before_processing() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);

        let mut registrations: Vec<Registration> = Vec::new(&env);
        for _ in 0u32..21u32 {
            let id = Address::generate(&env);
            registrations.push_back(Registration {
                contract_id: id,
                name: symbol_short!("svc"),
                check_interval: 60,
            });
        }

        client.register_contracts_batch(&owner, &registrations);
    }

    #[test]
    fn batch_skips_zero_check_interval() {
        let env = Env::default();
        set_timestamp(&env, 1_700_000_000);
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let bad = Address::generate(&env);
        let good = Address::generate(&env);

        let mut registrations: Vec<Registration> = Vec::new(&env);
        registrations.push_back(Registration {
            contract_id: bad.clone(),
            name: symbol_short!("bad"),
            check_interval: 0,
        });
        registrations.push_back(Registration {
            contract_id: good.clone(),
            name: symbol_short!("good"),
            check_interval: 60,
        });

        let results = client.register_contracts_batch(&owner, &registrations);

        assert_eq!(results.len(), 2);
        assert_eq!(
            results.get(0).unwrap(),
            BatchResult::Skipped(SString::from_str(
                &env,
                "check interval must be greater than zero",
            ))
        );
        assert_eq!(results.get(1).unwrap(), BatchResult::Success);
        assert_eq!(client.get_monitored_count(), 1);
    }

    #[test]
    fn empty_batch_returns_empty_results() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);

        let registrations: Vec<Registration> = Vec::new(&env);
        let results = client.register_contracts_batch(&owner, &registrations);

        assert_eq!(results.len(), 0);
        assert_eq!(client.get_monitored_count(), 0);
    }

    #[test]
    fn report_status_updates_record_and_emits_event() {
        let env = Env::default();
        set_timestamp(&env, 1_700_000_000);
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);

        set_timestamp(&env, 1_700_000_500);
        let metadata = SString::from_str(&env, "cpu=42");
        client.report_status(&owner, &monitored, &HealthStatus::Degraded, &metadata);

        let record = client.get_status(&monitored);
        assert_eq!(record.status, HealthStatus::Degraded);
        assert_eq!(record.last_check, 1_700_000_500);
    }

    #[test]
    #[should_panic(expected = "only admin or owner")]
    fn non_owner_cannot_report_status() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);
        let stranger = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);

        let metadata = SString::from_str(&env, "x");
        client.report_status(
            &stranger,
            &monitored,
            &HealthStatus::Unresponsive,
            &metadata,
        );
    }

    #[test]
    fn admin_can_report_status_for_any_contract() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);

        let metadata = SString::from_str(&env, "admin override");
        client.report_status(&admin, &monitored, &HealthStatus::Unresponsive, &metadata);
        assert_eq!(
            client.get_status(&monitored).status,
            HealthStatus::Unresponsive
        );
    }

    #[test]
    fn report_alert_appends_and_returns_history() {
        let env = Env::default();
        set_timestamp(&env, 1_700_000_000);
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);

        let msg1 = SString::from_str(&env, "queue backing up");
        let msg2 = SString::from_str(&env, "queue drained");
        client.report_alert(&owner, &monitored, &AlertSeverity::Warning, &msg1);
        set_timestamp(&env, 1_700_000_120);
        client.report_alert(&owner, &monitored, &AlertSeverity::Info, &msg2);

        let alerts = client.get_alerts(&monitored);
        assert_eq!(alerts.len(), 2);
        assert_eq!(alerts.get(0).unwrap().severity, AlertSeverity::Warning);
        assert_eq!(alerts.get(1).unwrap().severity, AlertSeverity::Info);
    }

    /// Registers a contract and reports three alerts at t=100, t=200, t=300.
    fn setup_alert_history(env: &Env) -> (WatchdogContractClient<'_>, Address) {
        let (_admin, client) = setup(env);
        let owner = Address::generate(env);
        let monitored = Address::generate(env);
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);
        for (ts, msg) in [(100u64, "first"), (200, "second"), (300, "third")] {
            set_timestamp(env, ts);
            client.report_alert(
                &owner,
                &monitored,
                &AlertSeverity::Warning,
                &SString::from_str(env, msg),
            );
        }
        (client, monitored)
    }

    #[test]
    fn get_alerts_since_zero_returns_all_alerts() {
        let env = Env::default();
        let (client, monitored) = setup_alert_history(&env);

        let since = client.get_alerts_since(&monitored, &0u64);
        assert_eq!(since.len(), 3);
        assert_eq!(since, client.get_alerts(&monitored));
    }

    #[test]
    fn get_alerts_since_returns_only_newer_alerts() {
        let env = Env::default();
        let (client, monitored) = setup_alert_history(&env);

        // Strictly greater than: the alert at exactly t=200 is excluded.
        let since = client.get_alerts_since(&monitored, &200u64);
        assert_eq!(since.len(), 1);
        assert_eq!(since.get(0).unwrap().timestamp, 300);
        assert_eq!(
            since.get(0).unwrap().message,
            SString::from_str(&env, "third")
        );

        let since = client.get_alerts_since(&monitored, &150u64);
        assert_eq!(since.len(), 2);
        assert_eq!(since.get(0).unwrap().timestamp, 200);
        assert_eq!(since.get(1).unwrap().timestamp, 300);
    }

    #[test]
    fn get_alerts_since_returns_empty_when_no_new_alerts() {
        let env = Env::default();
        let (client, monitored) = setup_alert_history(&env);

        assert_eq!(client.get_alerts_since(&monitored, &300u64).len(), 0);
        assert_eq!(client.get_alerts_since(&monitored, &u64::MAX).len(), 0);
    }

    #[test]
    fn get_alerts_since_unknown_contract_returns_empty() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let unknown = Address::generate(&env);

        assert_eq!(client.get_alerts_since(&unknown, &0u64).len(), 0);
    }

    #[test]
    fn deregister_removes_from_registry_and_state() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let a = Address::generate(&env);
        let b = Address::generate(&env);
        client.register_contract(&owner, &a, &symbol_short!("a"), &60u64);
        client.register_contract(&owner, &b, &symbol_short!("b"), &60u64);

        client.deregister_contract(&owner, &a);
        assert_eq!(client.get_monitored_count(), 1);
        let remaining = client.get_all_monitored();
        assert_eq!(remaining.len(), 1);
        assert_eq!(remaining.get(0).unwrap().contract_id, b);
    }

    #[test]
    #[should_panic(expected = "only admin or owner")]
    fn stranger_cannot_deregister() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let stranger = Address::generate(&env);
        let monitored = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("s"), &60u64);
        client.deregister_contract(&stranger, &monitored);
    }

    #[test]
    fn get_all_monitored_returns_every_registered_contract() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        for i in 0u32..5u32 {
            let id = Address::generate(&env);
            let name = if i % 2 == 0 {
                symbol_short!("even")
            } else {
                symbol_short!("odd")
            };
            client.register_contract(&owner, &id, &name, &60u64);
        }
        assert_eq!(client.get_all_monitored().len(), 5);
        assert_eq!(client.get_monitored_count(), 5);
    }

    #[test]
    fn get_monitored_page_returns_correct_page() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        for _ in 0u32..5u32 {
            let id = Address::generate(&env);
            let name = symbol_short!("test");
            client.register_contract(&owner, &id, &name, &60u64);
        }

        // Test first page
        let (page, total) = client.get_monitored_page(&0, &2);
        assert_eq!(page.len(), 2);
        assert_eq!(total, 5);

        // Test middle page
        let (page, total) = client.get_monitored_page(&2, &2);
        assert_eq!(page.len(), 2);
        assert_eq!(total, 5);

        // Test beyond end page
        let (page, total) = client.get_monitored_page(&10, &5);
        assert_eq!(page.len(), 0);
        assert_eq!(total, 5);
    }

    // ---- emergency stop (pause / unpause) ----------------------------------

    fn paused_error() -> soroban_sdk::Error {
        Error::ContractPaused.into()
    }

    #[test]
    fn pause_blocks_register_contract() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);

        client.pause(&admin);
        let res = client.try_register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);

        assert_eq!(res, Err(Ok(paused_error())));
        assert_eq!(client.get_monitored_count(), 0);
    }

    #[test]
    fn unpause_allows_register_contract_again() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);

        client.pause(&admin);
        client.unpause(&admin);
        assert!(!client.is_paused());

        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);
        assert_eq!(client.get_monitored_count(), 1);
        assert_eq!(client.get_status(&monitored).owner, owner);
    }

    #[test]
    #[should_panic(expected = "only admin or owner")]
    fn non_admin_cannot_pause() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let stranger = Address::generate(&env);
        // Auth is mocked, so this fails on the stored-admin check, not on
        // missing authorization.
        client.pause(&stranger);
    }

    #[test]
    #[should_panic(expected = "only admin or owner")]
    fn non_admin_cannot_unpause() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let stranger = Address::generate(&env);
        client.pause(&admin);
        client.unpause(&stranger);
    }

    #[test]
    fn contract_starts_unpaused() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);

        assert!(!client.is_paused());
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);
        assert_eq!(client.get_monitored_count(), 1);
    }

    #[test]
    fn pause_blocks_report_status_and_report_alert() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);

        client.pause(&admin);

        let metadata = SString::from_str(&env, "cpu=99");
        let res = client.try_report_status(&owner, &monitored, &HealthStatus::Degraded, &metadata);
        assert_eq!(res, Err(Ok(paused_error())));
        // The admin's override is blocked too.
        let res = client.try_report_status(&admin, &monitored, &HealthStatus::Degraded, &metadata);
        assert_eq!(res, Err(Ok(paused_error())));

        let msg = SString::from_str(&env, "down");
        let res = client.try_report_alert(&owner, &monitored, &AlertSeverity::Critical, &msg);
        assert_eq!(res, Err(Ok(paused_error())));

        assert_eq!(client.get_status(&monitored).status, HealthStatus::Healthy);
        assert_eq!(client.get_alerts(&monitored).len(), 0);
    }

    #[test]
    fn pause_blocks_batch_registration_and_deregistration() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);

        client.pause(&admin);

        let mut registrations: Vec<Registration> = Vec::new(&env);
        registrations.push_back(Registration {
            contract_id: Address::generate(&env),
            name: symbol_short!("new"),
            check_interval: 60,
        });
        let res = client.try_register_contracts_batch(&owner, &registrations);
        assert_eq!(res, Err(Ok(paused_error())));

        let res = client.try_deregister_contract(&owner, &monitored);
        assert_eq!(res, Err(Ok(paused_error())));

        assert_eq!(client.get_monitored_count(), 1);
        assert_eq!(client.get_status(&monitored).contract_id, monitored);
    }

    #[test]
    fn read_only_queries_work_while_paused() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);
        let msg = SString::from_str(&env, "heads up");
        client.report_alert(&owner, &monitored, &AlertSeverity::Info, &msg);

        client.pause(&admin);

        assert!(client.is_paused());
        assert_eq!(client.admin(), admin);
        assert_eq!(client.get_status(&monitored).owner, owner);
        assert_eq!(client.get_alerts(&monitored).len(), 1);
        assert_eq!(client.get_all_monitored().len(), 1);
        assert_eq!(client.get_monitored_count(), 1);
        let (page, total) = client.get_monitored_page(&0, &10);
        assert_eq!(page.len(), 1);
        assert_eq!(total, 1);
    }

    #[test]
    fn pause_and_unpause_are_idempotent() {
        let env = Env::default();
        let (admin, client) = setup(&env);

        client.unpause(&admin);
        assert!(!client.is_paused());

        client.pause(&admin);
        client.pause(&admin);
        assert!(client.is_paused());

        client.unpause(&admin);
        client.unpause(&admin);
        assert!(!client.is_paused());
    }

    #[test]
    fn threshold_multi_admin_proposal_flow_and_execution() {
        let env = Env::default();
        set_timestamp(&env, 1_000_000);
        let (admin, client) = setup(&env);

        let a1 = Address::generate(&env);
        let a2 = Address::generate(&env);
        let a3 = Address::generate(&env);
        let mut admins = Vec::new(&env);
        admins.push_back(a1.clone());
        admins.push_back(a2.clone());
        admins.push_back(a3.clone());

        let prop_id = client.set_admin_set(&admin, &admins, &2u32);
        let prop = client.get_proposal(&prop_id);
        assert_eq!(prop.approvals.len(), 1);
        assert_eq!(prop.executed, true);

        assert_eq!(client.admin_set().unwrap().threshold, 2);

        let pause_prop_id = client.propose_pause(&a1);
        let pause_prop = client.get_proposal(&pause_prop_id);
        assert_eq!(pause_prop.executed, false);

        client.approve_proposal(&a2, &pause_prop_id);
        let pause_prop_after = client.get_proposal(&pause_prop_id);
        assert_eq!(pause_prop_after.executed, true);
        assert_eq!(client.is_paused(), true);
    }

    #[test]
    #[should_panic(expected = "Error(Contract, #7)")]
    fn threshold_duplicate_approval_panics() {
        let env = Env::default();
        set_timestamp(&env, 1_000_000);
        let (admin, client) = setup(&env);

        let a1 = Address::generate(&env);
        let a2 = Address::generate(&env);
        let mut admins = Vec::new(&env);
        admins.push_back(a1.clone());
        admins.push_back(a2.clone());

        client.set_admin_set(&admin, &admins, &2u32);

        let prop_id = client.propose_pause(&a1);
        client.approve_proposal(&a1, &prop_id);
    }

    #[test]
    #[should_panic(expected = "Error(Contract, #6)")]
    fn threshold_expired_proposal_panics() {
        let env = Env::default();
        set_timestamp(&env, 1_000_000);
        let (admin, client) = setup(&env);

        let a1 = Address::generate(&env);
        let a2 = Address::generate(&env);
        let mut admins = Vec::new(&env);
        admins.push_back(a1.clone());
        admins.push_back(a2.clone());

        client.set_admin_set(&admin, &admins, &2u32);

        let prop_id = client.propose_pause(&a1);
        set_timestamp(&env, 2_000_000);
        client.approve_proposal(&a2, &prop_id);
    }

    #[test]
    fn pause_and_unpause_require_admin_auth() {
        let env = Env::default();
        let (admin, client) = setup(&env);

        client.pause(&admin);
        assert_eq!(
            env.auths(),
            std::vec![(
                admin.clone(),
                AuthorizedInvocation {
                    function: AuthorizedFunction::Contract((
                        client.address.clone(),
                        Symbol::new(&env, "pause"),
                        (admin.clone(),).into_val(&env),
                    )),
                    sub_invocations: std::vec![],
                }
            )]
        );

        client.unpause(&admin);
        assert_eq!(
            env.auths(),
            std::vec![(
                admin.clone(),
                AuthorizedInvocation {
                    function: AuthorizedFunction::Contract((
                        client.address.clone(),
                        Symbol::new(&env, "unpause"),
                        (admin.clone(),).into_val(&env),
                    )),
                    sub_invocations: std::vec![],
                }
            )]
        );
    }

    #[test]
    fn pause_without_admin_signature_fails() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        // Drop the blanket auth mock: the real admin address, but no signature.
        env.set_auths(&[]);

        assert!(client.try_pause(&admin).is_err());
        assert!(!client.is_paused());
    }

    // ---- two-step admin transfer -------------------------------------------

    #[test]
    fn admin_transfer_full_handshake_succeeds() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let new_admin = Address::generate(&env);

        assert_eq!(client.pending_admin(), None);
        client.propose_admin(&admin, &new_admin);
        assert_eq!(client.pending_admin(), Some(new_admin.clone()));
        // The nomination alone must not move admin authority.
        assert_eq!(client.admin(), admin);

        client.accept_admin(&new_admin);
        assert_eq!(client.admin(), new_admin);
        assert_eq!(client.pending_admin(), None);
    }

    #[test]
    fn accepted_admin_gains_privileges_and_old_admin_loses_them() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let new_admin = Address::generate(&env);

        client.propose_admin(&admin, &new_admin);
        client.accept_admin(&new_admin);

        // The new admin can exercise admin-only powers...
        client.pause(&new_admin);
        assert!(client.is_paused());
        client.unpause(&new_admin);
        assert!(!client.is_paused());

        // ...while the previous admin no longer can.
        assert!(client
            .try_propose_admin(&admin, &Address::generate(&env))
            .is_err());
        assert_eq!(client.admin(), new_admin);
    }

    #[test]
    #[should_panic(expected = "caller is not the pending admin")]
    fn non_pending_address_cannot_accept() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let pending = Address::generate(&env);
        let impostor = Address::generate(&env);

        client.propose_admin(&admin, &pending);
        // Auth is mocked, so this is rejected by the pending-admin check, not
        // by a missing authorization.
        client.accept_admin(&impostor);
    }

    #[test]
    fn impostor_accept_does_not_change_admin() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let pending = Address::generate(&env);
        let impostor = Address::generate(&env);

        client.propose_admin(&admin, &pending);
        assert!(client.try_accept_admin(&impostor).is_err());
        assert_eq!(client.admin(), admin);
        assert_eq!(client.pending_admin(), Some(pending));
    }

    #[test]
    #[should_panic(expected = "only admin or owner may perform this action")]
    fn non_admin_cannot_propose_admin() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let stranger = Address::generate(&env);
        client.propose_admin(&stranger, &Address::generate(&env));
    }

    #[test]
    #[should_panic(expected = "only admin or owner may perform this action")]
    fn non_admin_cannot_cancel_proposal() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let stranger = Address::generate(&env);
        client.propose_admin(&admin, &Address::generate(&env));
        client.cancel_admin_proposal(&stranger);
    }

    #[test]
    fn cancel_proposal_clears_pending_and_keeps_admin() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let pending = Address::generate(&env);

        client.propose_admin(&admin, &pending);
        client.cancel_admin_proposal(&admin);

        assert_eq!(client.pending_admin(), None);
        assert_eq!(client.admin(), admin);
        // A cancelled nominee can no longer accept.
        assert!(client.try_accept_admin(&pending).is_err());
    }

    #[test]
    #[should_panic(expected = "no admin transfer proposed")]
    fn accept_without_proposal_panics() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        client.accept_admin(&Address::generate(&env));
    }

    #[test]
    #[should_panic(expected = "no admin transfer proposed")]
    fn cancel_without_proposal_panics() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        client.cancel_admin_proposal(&admin);
    }

    #[test]
    #[should_panic(expected = "new admin is already the current admin")]
    fn proposing_current_admin_panics() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        client.propose_admin(&admin, &admin);
    }

    #[test]
    fn proposing_again_replaces_previous_nominee() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let first = Address::generate(&env);
        let second = Address::generate(&env);

        client.propose_admin(&admin, &first);
        client.propose_admin(&admin, &second);

        assert_eq!(client.pending_admin(), Some(second.clone()));
        assert!(client.try_accept_admin(&first).is_err());
        client.accept_admin(&second);
        assert_eq!(client.admin(), second);
    }

    #[test]
    fn propose_and_accept_require_expected_auth() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let new_admin = Address::generate(&env);

        client.propose_admin(&admin, &new_admin);
        assert_eq!(
            env.auths(),
            std::vec![(
                admin.clone(),
                AuthorizedInvocation {
                    function: AuthorizedFunction::Contract((
                        client.address.clone(),
                        Symbol::new(&env, "propose_admin"),
                        (admin.clone(), new_admin.clone()).into_val(&env),
                    )),
                    sub_invocations: std::vec![],
                }
            )]
        );

        client.accept_admin(&new_admin);
        assert_eq!(
            env.auths(),
            std::vec![(
                new_admin.clone(),
                AuthorizedInvocation {
                    function: AuthorizedFunction::Contract((
                        client.address.clone(),
                        Symbol::new(&env, "accept_admin"),
                        (new_admin.clone(),).into_val(&env),
                    )),
                    sub_invocations: std::vec![],
                }
            )]
        );
    }

    #[test]
    fn accept_without_nominee_signature_fails() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let new_admin = Address::generate(&env);

        client.propose_admin(&admin, &new_admin);
        // Drop the blanket auth mock: the nominee never signed.
        env.set_auths(&[]);

        assert!(client.try_accept_admin(&new_admin).is_err());
        assert_eq!(client.admin(), admin);
        assert_eq!(client.pending_admin(), Some(new_admin));
    }

    #[test]
    fn reads_extend_ttl_past_the_original_expiry() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);

        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);
        client.report_alert(
            &owner,
            &monitored,
            &AlertSeverity::Critical,
            &SString::from_str(&env, "down"),
        );

        // Just inside the extension threshold, so the next read must renew.
        env.ledger().with_mut(|li| {
            li.sequence_number = TTL_EXTEND_TO - TTL_THRESHOLD + 1;
        });

        assert_eq!(client.get_monitored_count(), 1);
        assert_eq!(client.get_status(&monitored).owner, owner);
        assert_eq!(client.get_alerts(&monitored).len(), 1);

        // Past the expiry the entries had at registration, but inside the
        // window renewed by the reads above.
        env.ledger().with_mut(|li| {
            li.sequence_number += TTL_EXTEND_TO / 2;
        });

        assert_eq!(client.admin(), admin);
        assert_eq!(client.get_monitored_count(), 1);
        assert_eq!(client.get_status(&monitored).owner, owner);
        assert_eq!(client.get_alerts(&monitored).len(), 1);
        assert_eq!(client.pending_admin(), None);
    }

    #[test]
    fn writes_extend_ttl_past_the_original_expiry() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);

        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);

        // Near expiry again, then renew through the mutating entry points.
        env.ledger().with_mut(|li| {
            li.sequence_number = TTL_EXTEND_TO - TTL_THRESHOLD + 1;
        });
        client.report_status(
            &owner,
            &monitored,
            &HealthStatus::Degraded,
            &SString::from_str(&env, "slow"),
        );
        client.report_alert(
            &owner,
            &monitored,
            &AlertSeverity::Warning,
            &SString::from_str(&env, "latency"),
        );

        env.ledger().with_mut(|li| {
            li.sequence_number += TTL_EXTEND_TO / 2;
        });

        let record = client.get_status(&monitored);
        assert_eq!(record.status, HealthStatus::Degraded);
        assert_eq!(client.get_alerts(&monitored).len(), 1);
        client.deregister_contract(&owner, &monitored);
        assert_eq!(client.get_monitored_count(), 0);
    }

    fn policy(_env: &Env, owner: &Address) -> MonitoringPolicy {
        MonitoringPolicy {
            warning_threshold: 3,
            critical_threshold: 10,
            expected_interval: 300,
            owner: owner.clone(),
        }
    }

    #[test]
    fn admin_sets_and_reads_policy_with_event() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);
        assert_eq!(client.get_policy(&monitored), None);

        let p = policy(&env, &owner);
        client.set_policy(&admin, &monitored, &p);
        assert_eq!(
            soroban_sdk::testutils::Events::all(&env.events())
                .events()
                .len(),
            1
        );
        assert_eq!(client.get_policy(&monitored), Some(p.clone()));

        let mut updated = p.clone();
        updated.critical_threshold = 20;
        client.set_policy(&admin, &monitored, &updated);
        assert_eq!(client.get_policy(&monitored), Some(updated));
    }

    #[test]
    fn set_policy_requires_admin_auth() {
        let env = Env::default();
        let (_admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);
        let p = policy(&env, &owner);
        // Not the admin, even though auth is mocked.
        assert!(client.try_set_policy(&owner, &monitored, &p).is_err());

        // Admin address without a signature is rejected.
        let env2 = Env::default();
        let id = env2.register(WatchdogContract {}, ());
        let c2 = WatchdogContractClient::new(&env2, &id);
        let admin2 = Address::generate(&env2);
        env2.mock_all_auths();
        c2.initialize(&admin2);
        let o2 = Address::generate(&env2);
        let m2 = Address::generate(&env2);
        c2.register_contract(&o2, &m2, &symbol_short!("svc"), &60u64);
        env2.set_auths(&[]);
        assert!(c2
            .try_set_policy(&admin2, &m2, &policy(&env2, &o2))
            .is_err());
    }

    #[test]
    fn invalid_policies_are_rejected() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);

        let mut bad = policy(&env, &owner);
        bad.warning_threshold = 0;
        assert!(client.try_set_policy(&admin, &monitored, &bad).is_err());
        let mut bad = policy(&env, &owner);
        bad.warning_threshold = 11;
        assert!(client.try_set_policy(&admin, &monitored, &bad).is_err());
        let mut bad = policy(&env, &owner);
        bad.critical_threshold = MAX_POLICY_THRESHOLD + 1;
        assert!(client.try_set_policy(&admin, &monitored, &bad).is_err());
        let mut bad = policy(&env, &owner);
        bad.expected_interval = 0;
        assert!(client.try_set_policy(&admin, &monitored, &bad).is_err());
        let mut bad = policy(&env, &owner);
        bad.expected_interval = MAX_POLICY_INTERVAL + 1;
        assert!(client.try_set_policy(&admin, &monitored, &bad).is_err());
        assert_eq!(client.get_policy(&monitored), None);
    }

    #[test]
    fn set_policy_requires_registered_contract() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let unknown = Address::generate(&env);
        assert!(client
            .try_set_policy(&admin, &unknown, &policy(&env, &owner))
            .is_err());
    }

    #[test]
    fn batch_policies_are_atomic_and_bounded() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let a = Address::generate(&env);
        let b = Address::generate(&env);
        client.register_contract(&owner, &a, &symbol_short!("a"), &60u64);
        client.register_contract(&owner, &b, &symbol_short!("b"), &60u64);

        let mut entries: Vec<PolicyEntry> = Vec::new(&env);
        entries.push_back(PolicyEntry {
            contract_id: a.clone(),
            policy: policy(&env, &owner),
        });
        entries.push_back(PolicyEntry {
            contract_id: b.clone(),
            policy: policy(&env, &owner),
        });
        client.set_policies_batch(&admin, &entries);
        let got = client.get_policies_batch(&Vec::from_array(&env, [a.clone(), b.clone()]));
        assert_eq!(got.len(), 2);
        assert_eq!(got.get(0).unwrap(), Some(policy(&env, &owner)));

        // One invalid entry rolls the whole batch back.
        let c = Address::generate(&env);
        client.register_contract(&owner, &c, &symbol_short!("c"), &60u64);
        let mut bad = policy(&env, &owner);
        bad.expected_interval = 0;
        let mut entries: Vec<PolicyEntry> = Vec::new(&env);
        entries.push_back(PolicyEntry {
            contract_id: c.clone(),
            policy: policy(&env, &owner),
        });
        entries.push_back(PolicyEntry {
            contract_id: a.clone(),
            policy: bad,
        });
        assert!(client.try_set_policies_batch(&admin, &entries).is_err());
        assert_eq!(client.get_policy(&c), None);

        // Empty and oversized batches are rejected.
        assert!(client
            .try_set_policies_batch(&admin, &Vec::new(&env))
            .is_err());
        let mut big: Vec<PolicyEntry> = Vec::new(&env);
        for _ in 0..(MAX_BATCH_SIZE + 1) {
            big.push_back(PolicyEntry {
                contract_id: a.clone(),
                policy: policy(&env, &owner),
            });
        }
        assert!(client.try_set_policies_batch(&admin, &big).is_err());
    }

    #[test]
    fn policy_ttl_is_extended_and_removed_on_deregister() {
        let env = Env::default();
        let (admin, client) = setup(&env);
        let owner = Address::generate(&env);
        let monitored = Address::generate(&env);
        client.register_contract(&owner, &monitored, &symbol_short!("svc"), &60u64);
        client.set_policy(&admin, &monitored, &policy(&env, &owner));

        env.ledger().with_mut(|li| {
            li.sequence_number = TTL_EXTEND_TO - TTL_THRESHOLD + 1;
        });
        assert!(client.get_policy(&monitored).is_some());
        env.ledger().with_mut(|li| {
            li.sequence_number += TTL_EXTEND_TO / 2;
        });
        assert!(client.get_policy(&monitored).is_some());

        client.deregister_contract(&owner, &monitored);
        assert_eq!(client.get_policy(&monitored), None);
    }
}
