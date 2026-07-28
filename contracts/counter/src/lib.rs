#![no_std]

use soroban_sdk::{contract, contractevent, contractimpl, contracttype, symbol_short, Env, Symbol};

#[derive(Clone)]
#[contracttype]
#[repr(u32)]
enum DataKey {
    Counter = 0,
    Label = 1,
}

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct IncrementEvent {
    #[topic]
    pub by: u32,
    pub new_count: u32,
}

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResetEvent {
    pub reason: Symbol,
}

#[contractevent]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SetLabelEvent {
    #[topic]
    pub label: Symbol,
    pub note: Symbol,
}

#[contract]
pub struct CounterContract;

#[contractimpl]
impl CounterContract {
    pub fn increment(env: Env, by: u32) -> u32 {
        let mut count: u32 = env
            .storage()
            .persistent()
            .get(&DataKey::Counter)
            .unwrap_or(0);
        count = count.saturating_add(by);
        env.storage().persistent().set(&DataKey::Counter, &count);
        IncrementEvent {
            by,
            new_count: count,
        }
        .publish(&env);
        count
    }

    pub fn get(env: Env) -> u32 {
        env.storage()
            .persistent()
            .get(&DataKey::Counter)
            .unwrap_or(0)
    }

    pub fn reset(env: Env) {
        env.storage().persistent().set(&DataKey::Counter, &0u32);
        ResetEvent {
            reason: symbol_short!("cleared"),
        }
        .publish(&env);
    }

    pub fn set_label(env: Env, label: Symbol) {
        env.storage().temporary().set(&DataKey::Label, &label);
        env.storage()
            .temporary()
            .extend_ttl(&DataKey::Label, 1, 1000);
        SetLabelEvent {
            label: label.clone(),
            note: symbol_short!("label_set"),
        }
        .publish(&env);
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use soroban_sdk::{symbol_short, testutils::Events, Env, Event, Symbol};

    #[test]
    fn counter_round_trip_and_events() {
        let env = Env::default();
        let contract_id = env.register(CounterContract {}, ());
        let client = CounterContractClient::new(&env, &contract_id);

        assert_eq!(client.get(), 0u32);

        assert_eq!(client.increment(&3u32), 3u32);
        let events = env.events().all();
        assert_eq!(events.events().len(), 1);
        assert_eq!(
            events.events()[0],
            IncrementEvent {
                by: 3,
                new_count: 3
            }
            .to_xdr(&env, &contract_id)
        );

        assert_eq!(client.get(), 3u32);

        client.reset();
        let events = env.events().all();
        assert_eq!(events.events().len(), 1);
        assert_eq!(
            events.events()[0],
            ResetEvent {
                reason: symbol_short!("cleared")
            }
            .to_xdr(&env, &contract_id)
        );
        assert_eq!(client.get(), 0u32);

        let label = Symbol::new(&env, "alpha");
        client.set_label(&label);
        let events = env.events().all();
        assert_eq!(events.events().len(), 1);
        assert_eq!(
            events.events()[0],
            SetLabelEvent {
                label: label.clone(),
                note: symbol_short!("label_set"),
            }
            .to_xdr(&env, &contract_id)
        );
    }
}
