#![cfg(feature = "blocking")]

//! Integration tests for the blocking client.
//!
//! The blocking transport is exercised against the same recorded fixtures as
//! the asynchronous suite, so both transports are proven to send identical
//! requests.

use std::time::Duration;

use httpmock::prelude::*;
use sorolens_sdk::*;

const CONTRACT_ID: &str = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4";

fn client(server: &MockServer) -> BlockingClient {
    BlockingClient::builder()
        .base_url(server.base_url())
        .api_key("sl_test_key")
        .timeout(Duration::from_secs(10))
        .build()
        .expect("client builds")
}

#[test]
fn blocking_client_reads_contracts_and_stats() {
    let server = MockServer::start();
    let contracts = server.mock(|when, then| {
        when.method(GET)
            .path("/api/v1/contracts")
            .query_param("network", "testnet")
            .header_exists("authorization");
        then.status(200)
            .header("content-type", "application/json")
            .body(include_str!("fixtures/contracts_page.json"));
    });
    let stats_mock = server.mock(|when, then| {
        when.method(GET).path("/api/v1/stats/global");
        then.status(200)
            .header("content-type", "application/json")
            .body(include_str!("fixtures/global_stats.json"));
    });

    let client = client(&server);

    let page = client
        .list_contracts(&ListContractsParams::default().network(Network::Testnet))
        .unwrap();
    assert_eq!(page.contracts.len(), 2);
    assert_eq!(page.contracts[0].network, Network::Testnet);

    let stats = client.get_global_stats().unwrap();
    assert_eq!(stats.tracked_contracts, 3);

    contracts.assert();
    stats_mock.assert();
}

#[test]
fn blocking_client_registers_a_contract() {
    let server = MockServer::start();
    let mock = server.mock(|when, then| {
        when.method(POST)
            .path("/api/v1/contracts")
            .json_body(serde_json::json!({
                "id": CONTRACT_ID,
                "network": "mainnet",
            }));
        then.status(201)
            .header("content-type", "application/json")
            .body(include_str!("fixtures/contract.json"));
    });

    let contract = client(&server)
        .register_contract(&RegisterContractRequest::new(CONTRACT_ID, Network::Mainnet))
        .unwrap();
    assert_eq!(contract.id, CONTRACT_ID);
    mock.assert();
}

#[test]
fn blocking_client_reads_watchdog_health_checks() {
    let server = MockServer::start();
    let mock = server.mock(|when, then| {
        when.method(GET)
            .path(format!("/api/v1/watchdog/contracts/{CONTRACT_ID}/health"));
        then.status(200)
            .header("content-type", "application/json")
            .body(include_str!("fixtures/health_checks.json"));
    });

    let checks = client(&server)
        .list_contract_health_checks(CONTRACT_ID, &ListContractHealthChecksParams::default())
        .unwrap();
    assert_eq!(checks.health_checks.len(), 1);
    assert_eq!(checks.health_checks[0].status, "healthy");
    mock.assert();
}

#[test]
fn blocking_client_surfaces_api_errors() {
    let server = MockServer::start();
    let mock = server.mock(|when, then| {
        when.method(GET)
            .path(format!("/api/v1/contracts/{CONTRACT_ID}"));
        then.status(404)
            .header("content-type", "application/json")
            .body(include_str!("fixtures/error_not_found.json"));
    });

    let err = client(&server).get_contract(CONTRACT_ID).unwrap_err();
    assert_eq!(err.status(), Some(404));
    assert_eq!(err.api_error().unwrap().message, "contract not found");
    mock.assert();
}

#[test]
fn blocking_client_validates_before_sending() {
    let client = BlockingClient::builder()
        .base_url("http://127.0.0.1:1")
        .timeout(Duration::from_millis(50))
        .build()
        .unwrap();

    assert!(matches!(
        client.get_contract(""),
        Err(Error::InvalidRequest(_))
    ));
    assert!(matches!(
        client.get_contract_snapshot(CONTRACT_ID, &GetContractSnapshotParams::new(-1)),
        Err(Error::InvalidRequest(_))
    ));
}
