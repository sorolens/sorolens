#![cfg(test)]

use super::*;

#[test]
fn test_watchdog_basic_invariants() {
    let env = Env::default();
    env.mock_all_auths();
    let contract_id = env.register(WatchdogContract, ());
    let client = WatchdogContractClient::new(&env, &contract_id);

    let admin = Address::generate(&env);
    client.initialize(&admin);

    let target = Address::generate(&env);
    let service = Symbol::new(&env, "auth-svc");

    assert!(!client.is_registered(&target));
    client.register_contract(&admin, &target, &service, &60);
    assert!(client.is_registered(&target));

    client.deregister_contract(&admin, &target);
    assert!(!client.is_registered(&target));
}

#[test]
fn prop_registry_and_pause_invariants_fuzz() {
    // Pseudo-random deterministic sequence generator (LCG) to simulate property-based test runs without external dependencies.
    let mut seed: u64 = 123456789;
    let mut next_u8 = || {
        seed = seed.wrapping_mul(1103515245).wrapping_add(12345);
        ((seed / 65536) % 256) as u8
    };

    for run in 0..50 {
        let env = Env::default();
        env.mock_all_auths();
        let contract_id = env.register(WatchdogContract, ());
        let client = WatchdogContractClient::new(&env, &contract_id);

        let admin = Address::generate(&env);
        client.initialize(&admin);

        let target = Address::generate(&env);
        let service = Symbol::new(&env, "prop-svc");
        let mut paused = false;

        let num_actions = (next_u8() % 15) + 5;
        for _ in 0..num_actions {
            let action = next_u8() % 3;
            match action {
                0 => {
                    let res = client.try_register_contract(&admin, &target, &service, &60);
                    if paused {
                        assert!(res.is_err(), "run {} step register while paused should fail", run);
                    } else {
                        assert!(res.is_ok(), "run {} step register should succeed", run);
                        assert!(client.is_registered(&target));
                    }
                }
                1 => {
                    let res = client.try_deregister_contract(&admin, &target);
                    if paused {
                        assert!(res.is_err(), "run {} step deregister while paused should fail", run);
                    } else {
                        assert!(res.is_ok(), "run {} step deregister should succeed", run);
                        assert!(!client.is_registered(&target));
                    }
                }
                2 => {
                    if paused {
                        let res = client.try_unpause(&admin);
                        if res.is_ok() {
                            paused = false;
                        }
                    } else {
                        let res = client.try_pause(&admin);
                        if res.is_ok() {
                            paused = true;
                        }
                    }
                }
                _ => {}
            }
        }
    }
}
