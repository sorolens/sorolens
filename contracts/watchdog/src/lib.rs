#![no_std]

//! Sorolens Watchdog contract.
//!
//! On-chain health tracking for other Soroban contracts. Contract owners
//! register a contract they operate, then periodically push health status
//! updates or alerts. The Sorolens indexer subscribes to the events emitted
//! here and materialises them into the dashboard.

use soroban_sdk::{
    contract, contracterror, contractevent, contractimpl, contracttype, panic_with_error,
    symbol_short, Address, Env, String, Symbol, Vec,
};

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

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
#[derive(Clone)]
enum DataKey {
    Admin,
    Registry,        // Vec<Address>: list of monitored contract ids
    Health(Address), // ContractHealth by contract id
    Alerts(Address), // Vec<Alert> by contract id
    Paused,          // bool: emergency stop flag; absent means not paused
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

// ---------------------------------------------------------------------------
// Contract
// ---------------------------------------------------------------------------

#[contract]
pub struct WatchdogContract;

const MAX_ALERTS_PER_CONTRACT: u32 = 100;
const MAX_BATCH_SIZE: u32 = 20;

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
    }

    /// Lift a pause set by `pause`. Admin only. Unpausing a contract that is
    /// not paused is a no-op.
    pub fn unpause(env: Env, admin: Address) {
        admin.require_auth();
        Self::require_admin_only(&env, &admin);
        env.storage().instance().remove(&DataKey::Paused);
    }

    // ---- read-only queries -------------------------------------------------

    pub fn get_status(env: Env, contract_id: Address) -> ContractHealth {
        env.storage()
            .persistent()
            .get(&DataKey::Health(contract_id))
            .unwrap_or_else(|| panic!("contract not registered"))
    }

    /// Gets a page of monitored contracts.
    pub fn get_monitored_page(env: Env, start: u32, limit: u32) -> (Vec<ContractHealth>, u32) {
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
                    if let Some(record) = env.storage().persistent().get::<_, ContractHealth>(&DataKey::Health(id.clone())) {
                        page.push_back(record);
                    }
                }
            }
        }
        (page, total)
    }

    pub fn get_all_monitored(env: Env) -> Vec<ContractHealth> {
        let registry: Vec<Address> = env
            .storage()
            .instance()
            .get(&DataKey::Registry)
            .unwrap_or_else(|| Vec::new(&env));
        let mut out: Vec<ContractHealth> = Vec::new(&env);
        for id in registry.iter() {
            if let Some(record) = env.storage().persistent().get(&DataKey::Health(id)) {
                out.push_back(record);
            }
        }
        out
    }

    pub fn get_alerts(env: Env, contract_id: Address) -> Vec<Alert> {
        env.storage()
            .persistent()
            .get(&DataKey::Alerts(contract_id))
            .unwrap_or_else(|| Vec::new(&env))
    }

    pub fn get_monitored_count(env: Env) -> u32 {
        let registry: Vec<Address> = env
            .storage()
            .instance()
            .get(&DataKey::Registry)
            .unwrap_or_else(|| Vec::new(&env));
        registry.len()
    }

    pub fn admin(env: Env) -> Address {
        env.storage()
            .instance()
            .get(&DataKey::Admin)
            .unwrap_or_else(|| panic!("not initialized"))
    }

    pub fn is_paused(env: Env) -> bool {
        env.storage()
            .instance()
            .get(&DataKey::Paused)
            .unwrap_or(false)
    }

    // ---- helpers -----------------------------------------------------------

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
}
