#![cfg(any(feature = "async", feature = "blocking"))]

use wiremock::matchers::{method, path, query_param};
use wiremock::{Mock, MockServer, ResponseTemplate};

#[cfg(feature = "async")]
#[tokio::test]
async fn resolve_unknown_label_returns_api_error() {
    let server = MockServer::start().await;

    Mock::given(method("GET"))
        .and(path("/api/v1/resolve"))
        .and(query_param("query", "unknown label"))
        .respond_with(ResponseTemplate::new(404).set_body_json(serde_json::json!({
            "error": "label not found"
        })))
        .expect(1)
        .mount(&server)
        .await;

    let sdk = sorolens_sdk::Client::builder()
        .base_url(server.uri())
        .build()
        .unwrap();

    let error = sdk.resolve_label("unknown label").await.unwrap_err();

    assert_eq!(error.status(), Some(404));
    assert_eq!(error.api_error().unwrap().message, "label not found");
}

#[cfg(feature = "blocking")]
#[tokio::test]
async fn blocking_client_downloads_an_svg_badge() {
    let server = MockServer::start().await;
    let contract_id = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4";
    let badge_path = format!("/api/v1/reports/{contract_id}/badge.svg");
    let svg = r#"<svg xmlns="http://www.w3.org/2000/svg"></svg>"#;

    Mock::given(method("GET"))
        .and(path(badge_path.as_str()))
        .and(query_param("month", "2026-01"))
        .respond_with(ResponseTemplate::new(200).set_body_raw(svg, "image/svg+xml"))
        .expect(1)
        .mount(&server)
        .await;

    let base_url = server.uri();
    let badge = tokio::task::spawn_blocking(move || {
        let sdk = sorolens_sdk::BlockingClient::builder()
            .base_url(base_url)
            .build()
            .unwrap();

        sdk.get_contract_sla_badge(contract_id, Some("2026-01"))
            .unwrap()
    })
    .await
    .unwrap();

    assert_eq!(badge.as_slice(), svg.as_bytes());
}
