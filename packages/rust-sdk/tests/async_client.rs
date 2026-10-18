#![cfg(feature = "async")]

//! Integration tests for the asynchronous client.
//!
//! Every payload is a recorded response shaped exactly like the schemas in
//! `docs/openapi.yaml`; `httpmock` serves them locally so the suite never
//! touches the network.

use httpmock::prelude::*;
use httpmock::{Method, Mock};
use sorolens_sdk::*;

const CONTRACT_ID: &str = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4";
const API_KEY: &str = "sl_test_key";

fn client(server: &MockServer) -> Client {
    Client::builder()
        .base_url(server.base_url())
        .api_key(API_KEY)
        .build()
        .expect("client builds")
}

async fn json_mock<'a>(
    server: &'a MockServer,
    method: Method,
    path: &str,
    body: &'static str,
) -> Mock<'a> {
    server
        .mock_async(move |when, then| {
            when.method(method).path(path);
            then.status(200)
                .header("content-type", "application/json")
                .body(body);
        })
        .await
}

#[tokio::test]
async fn health_and_readiness_probes() {
    let server = MockServer::start_async().await;
    let health_mock = json_mock(
        &server,
        GET,
        "/health",
        include_str!("fixtures/health.json"),
    )
    .await;
    let readyz_mock = json_mock(
        &server,
        GET,
        "/readyz",
        include_str!("fixtures/readyz.json"),
    )
    .await;

    let client = client(&server);
    assert_eq!(client.health().await.unwrap().status, "ok");
    assert_eq!(client.readyz().await.unwrap().status, "ok");

    health_mock.assert_async().await;
    readyz_mock.assert_async().await;
}

#[tokio::test]
async fn global_stats_decode() {
    let server = MockServer::start_async().await;
    let stats_mock = json_mock(
        &server,
        GET,
        "/api/v1/stats/global",
        include_str!("fixtures/global_stats.json"),
    )
    .await;

    let stats = client(&server).get_global_stats().await.unwrap();
    assert_eq!(stats.tracked_contracts, 3);
    assert_eq!(stats.total_events, 128);
    assert_eq!(stats.total_invocations, 42);
    assert_eq!(stats.total_storage_entries, 57);
    stats_mock.assert_async().await;
}

#[tokio::test]
async fn list_contracts_applies_filters_and_bearer_auth() {
    let server = MockServer::start_async().await;
    let list_mock = server
        .mock_async(|when, then| {
            when.method(GET)
                .path("/api/v1/contracts")
                .query_param("limit", "10")
                .query_param("network", "testnet")
                .query_param("status", "active")
                .header_exists("authorization");
            then.status(200)
                .header("content-type", "application/json")
                .body(include_str!("fixtures/contracts_page.json"));
        })
        .await;

    let params = ListContractsParams::default()
        .limit(10)
        .network(Network::Testnet)
        .status(ContractStatus::Active);
    let page = client(&server).list_contracts(&params).await.unwrap();

    assert_eq!(page.contracts.len(), 2);
    assert_eq!(page.contracts[0].status, ContractStatus::Active);
    assert!(page.contracts[0].backfill_complete_at.is_some());
    assert_eq!(page.contracts[1].status, ContractStatus::Pending);
    assert!(page.contracts[1].backfill_complete_at.is_none());
    assert_eq!(page.next_cursor, "Y3Vyc29yLTI=");
    list_mock.assert_async().await;
}

#[tokio::test]
async fn get_contract_surfaces_structured_api_errors() {
    let server = MockServer::start_async().await;
    let error_mock = server
        .mock_async(|when, then| {
            when.method(GET)
                .path(format!("/api/v1/contracts/{CONTRACT_ID}"));
            then.status(404)
                .header("content-type", "application/json")
                .body(include_str!("fixtures/error_not_found.json"));
        })
        .await;

    let err = client(&server).get_contract(CONTRACT_ID).await.unwrap_err();
    assert_eq!(err.status(), Some(404));
    let api = err.api_error().expect("structured api error");
    assert_eq!(api.code(), Some("NOT_FOUND"));
    assert_eq!(api.message, "contract not found");
    assert_eq!(api.request_id(), Some("req_01HQZ8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8"));
    assert_eq!(
        api.to_string(),
        "HTTP 404 NOT_FOUND: contract not found (request_id: req_01HQZ8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8)"
    );
    error_mock.assert_async().await;
}

#[tokio::test]
async fn register_contract_posts_a_json_body() {
    let server = MockServer::start_async().await;
    let register_mock = server
        .mock_async(|when, then| {
            when.method(POST)
                .path("/api/v1/contracts")
                .json_body(serde_json::json!({
                    "id": CONTRACT_ID,
                    "network": "testnet",
                    "label": "fixture-counter",
                }));
            then.status(201)
                .header("content-type", "application/json")
                .body(include_str!("fixtures/contract.json"));
        })
        .await;

    let request =
        RegisterContractRequest::new(CONTRACT_ID, Network::Testnet).with_label("fixture-counter");
    let contract = client(&server).register_contract(&request).await.unwrap();
    assert_eq!(contract.id, CONTRACT_ID);
    assert_eq!(contract.label, "fixture-counter");
    register_mock.assert_async().await;
}

#[tokio::test]
async fn events_list_and_stream_endpoints() {
    let server = MockServer::start_async().await;
    let list_mock = server
        .mock_async(|when, then| {
            when.method(GET)
                .path(format!("/api/v1/contracts/{CONTRACT_ID}/events"))
                .query_param("type", "contract")
                .query_param("from", "1234000");
            then.status(200)
                .header("content-type", "application/json")
                .body(include_str!("fixtures/events_page.json"));
        })
        .await;
    let stream_mock = json_mock(
        &server,
        GET,
        &format!("/api/v1/contracts/{CONTRACT_ID}/stream"),
        include_str!("fixtures/events_page.json"),
    )
    .await;

    let client = client(&server);
    let params = ListContractEventsParams::default()
        .event_type("contract")
        .ledger_range(1234000, 1235000);
    let page = client
        .list_contract_events(CONTRACT_ID, &params)
        .await
        .unwrap();
    assert_eq!(page.events.len(), 1);
    assert_eq!(page.events[0].event_type, "contract");
    assert!(page.events[0].in_successful_call);
    assert_eq!(page.events[0].topic_decoded[0]["symbol"], "transfer");

    let recent = client.stream_contract_events(CONTRACT_ID).await.unwrap();
    assert_eq!(recent.events.len(), 1);

    list_mock.assert_async().await;
    stream_mock.assert_async().await;
}

#[tokio::test]
async fn invocations_decode_resource_usage() {
    let server = MockServer::start_async().await;
    let invocations_mock = json_mock(
        &server,
        GET,
        &format!("/api/v1/contracts/{CONTRACT_ID}/invocations"),
        include_str!("fixtures/invocations_page.json"),
    )
    .await;

    let page = client(&server)
        .list_contract_invocations(CONTRACT_ID, &ListContractInvocationsParams::default())
        .await
        .unwrap();

    assert_eq!(page.invocations.len(), 2);
    assert_eq!(page.invocations[0].status, InvocationStatus::Success);
    assert_eq!(page.invocations[0].function_name, "transfer");
    assert_eq!(page.invocations[0].cpu_insn, 450_000);
    assert_eq!(page.invocations[1].status, InvocationStatus::Failed);
    assert_eq!(page.invocations[1].result_decoded, serde_json::Value::Null);
    assert_eq!(page.next_cursor, "");
    invocations_mock.assert_async().await;
}

#[tokio::test]
async fn storage_list_and_snapshot() {
    let server = MockServer::start_async().await;
    let storage_mock = json_mock(
        &server,
        GET,
        &format!("/api/v1/contracts/{CONTRACT_ID}/storage"),
        include_str!("fixtures/storage_page.json"),
    )
    .await;
    let snapshot_mock = server
        .mock_async(|when, then| {
            when.method(GET)
                .path(format!("/api/v1/contracts/{CONTRACT_ID}/snapshot"))
                .query_param("ledger", "1234600");
            then.status(200)
                .header("content-type", "application/json")
                .body(include_str!("fixtures/snapshot.json"));
        })
        .await;

    let client = client(&server);
    let page = client
        .list_contract_storage(CONTRACT_ID, &ListContractStorageParams::default())
        .await
        .unwrap();
    assert_eq!(page.storage.len(), 1);
    assert_eq!(page.storage[0].durability, Durability::Persistent);
    assert_eq!(page.storage[0].decoded_key().unwrap()["symbol"], "balance");

    let snap = client
        .get_contract_snapshot(CONTRACT_ID, &GetContractSnapshotParams::new(1234600))
        .await
        .unwrap();
    assert_eq!(snap.ledger, 1234600);
    assert_eq!(snap.storage[0].status, StorageStatus::Archived);
    assert_eq!(snap.last_event.as_ref().unwrap().event_type, "diagnostic");

    storage_mock.assert_async().await;
    snapshot_mock.assert_async().await;
}

#[tokio::test]
async fn contract_stats_and_forecast() {
    let server = MockServer::start_async().await;
    let stats_mock = server
        .mock_async(|when, then| {
            when.method(GET)
                .path(format!("/api/v1/contracts/{CONTRACT_ID}/stats"))
                .query_param("window", "30d");
            then.status(200)
                .header("content-type", "application/json")
                .body(include_str!("fixtures/contract_stats.json"));
        })
        .await;
    let forecast_mock = json_mock(
        &server,
        GET,
        &format!("/api/v1/contracts/{CONTRACT_ID}/forecast"),
        include_str!("fixtures/forecast.json"),
    )
    .await;

    let client = client(&server);
    let stats = client
        .get_contract_stats(
            CONTRACT_ID,
            &GetContractStatsParams::default().window("30d"),
        )
        .await
        .unwrap();
    assert_eq!(stats.window_duration, "24h");
    assert_eq!(stats.last_synced_ledger, 1234600);

    let forecast = client
        .get_contract_forecast(
            CONTRACT_ID,
            &GetContractForecastParams::default().horizon(30),
        )
        .await
        .unwrap();
    assert_eq!(forecast.lookback_days, 30);
    assert_eq!(forecast.series.len(), 2);
    assert_eq!(forecast.series[0].metric, ForecastMetric::Fees);
    assert_eq!(forecast.series[0].points[0].value, 1.5);
    assert_eq!(forecast.series[0].points[0].date.to_string(), "2026-02-01");

    stats_mock.assert_async().await;
    forecast_mock.assert_async().await;
}

#[tokio::test]
async fn watchdog_summary_and_alerts() {
    let server = MockServer::start_async().await;
    let summary_mock = json_mock(
        &server,
        GET,
        "/api/v1/watchdog/stats",
        include_str!("fixtures/watchdog_stats.json"),
    )
    .await;
    let alerts_mock = server
        .mock_async(|when, then| {
            when.method(GET)
                .path("/api/v1/watchdog/alerts")
                .query_param("severity", "Critical");
            then.status(200)
                .header("content-type", "application/json")
                .body(include_str!("fixtures/watchdog_alerts.json"));
        })
        .await;

    let client = client(&server);
    let stats = client
        .get_watchdog_stats(&GetWatchdogStatsParams::default())
        .await
        .unwrap();
    assert_eq!(stats.total_monitored, 12);
    assert_eq!(stats.critical_alerts, 1);

    let alerts = client
        .list_watchdog_alerts(&ListWatchdogAlertsParams::default().severity(Severity::Critical))
        .await
        .unwrap();
    assert_eq!(alerts.alerts.len(), 2);
    assert_eq!(alerts.alerts[0].severity, Severity::Critical);
    assert_eq!(alerts.alerts[1].severity, Severity::Info);

    summary_mock.assert_async().await;
    alerts_mock.assert_async().await;
}

#[tokio::test]
async fn watchdog_monitored_contracts_and_health_checks() {
    let server = MockServer::start_async().await;
    let page_mock = json_mock(
        &server,
        GET,
        "/api/v1/watchdog/contracts",
        include_str!("fixtures/monitored_contracts_page.json"),
    )
    .await;
    let contract_mock = json_mock(
        &server,
        GET,
        &format!("/api/v1/watchdog/contracts/{CONTRACT_ID}"),
        include_str!("fixtures/monitored_contract.json"),
    )
    .await;
    let checks_mock = json_mock(
        &server,
        GET,
        &format!("/api/v1/watchdog/contracts/{CONTRACT_ID}/health"),
        include_str!("fixtures/health_checks.json"),
    )
    .await;
    let contract_alerts_mock = json_mock(
        &server,
        GET,
        &format!("/api/v1/watchdog/contracts/{CONTRACT_ID}/alerts"),
        include_str!("fixtures/watchdog_alerts.json"),
    )
    .await;

    let client = client(&server);

    let monitored = client
        .list_monitored_contracts(&ListMonitoredContractsParams::default())
        .await
        .unwrap();
    assert_eq!(monitored.contracts.len(), 1);
    assert_eq!(monitored.contracts[0].status, "healthy");
    assert_eq!(monitored.contracts[0].network, Network::Testnet);

    let contract = client.get_monitored_contract(CONTRACT_ID).await.unwrap();
    assert_eq!(contract.status, "degraded");
    assert!(contract.last_check.is_none());

    let checks = client
        .list_contract_health_checks(CONTRACT_ID, &ListContractHealthChecksParams::default())
        .await
        .unwrap();
    assert_eq!(checks.health_checks.len(), 1);
    assert_eq!(checks.health_checks[0].metadata, "ledger_lag=2");

    let alerts = client
        .list_contract_alerts(CONTRACT_ID, &ListContractAlertsParams::default())
        .await
        .unwrap();
    assert_eq!(alerts.alerts.len(), 2);

    page_mock.assert_async().await;
    contract_mock.assert_async().await;
    checks_mock.assert_async().await;
    contract_alerts_mock.assert_async().await;
}

#[tokio::test]
async fn invalid_input_is_rejected_before_the_request() {
    // Nothing listens on port 1: if validation did not short-circuit, these
    // calls would fail with a transport error instead.
    let client = Client::builder()
        .base_url("http://127.0.0.1:1")
        .build()
        .unwrap();

    assert!(matches!(
        client.get_contract("  ").await,
        Err(Error::InvalidRequest(_))
    ));
    assert!(matches!(
        client
            .get_contract_forecast(
                CONTRACT_ID,
                &GetContractForecastParams::default().horizon(400)
            )
            .await,
        Err(Error::InvalidRequest(_))
    ));
    assert!(matches!(
        client
            .list_watchdog_alerts(&ListWatchdogAlertsParams::default().limit(9_999))
            .await,
        Err(Error::InvalidRequest(_))
    ));
}

#[tokio::test]
async fn rate_limits_are_surfaced_as_api_errors() {
    let server = MockServer::start_async().await;
    let rate_limited_mock = server
        .mock_async(|when, then| {
            when.method(GET).path("/api/v1/stats/global");
            then.status(429)
                .header("content-type", "application/json")
                .body(include_str!("fixtures/error_rate_limited.json"));
        })
        .await;

    let err = client(&server).get_global_stats().await.unwrap_err();
    assert_eq!(err.status(), Some(429));
    assert_eq!(err.api_error().unwrap().code(), Some("RATE_LIMITED"));
    rate_limited_mock.assert_async().await;
}
