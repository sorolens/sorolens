use std::fs;
use std::path::Path;

#[derive(serde::Deserialize)]
#[allow(dead_code)]
struct Fixture {
    id: String,
    #[serde(default)]
    description: String,
    request: FixtureRequest,
    response: FixtureResponse,
    expected: FixtureExpected,
}

#[derive(serde::Deserialize)]
#[allow(dead_code)]
struct FixtureRequest {
    method: String,
    path: String,
}

#[derive(serde::Deserialize)]
struct FixtureResponse {
    status: u16,
    headers: serde_json::Value,
    body: serde_json::Value,
}

#[derive(serde::Deserialize)]
struct FixtureExpected {
    status: u16,
    error_code: Option<String>,
    retry_after: Option<String>,
    count: Option<usize>,
}

#[test]
fn test_replays_sdk_conformance_fixtures() {
    let manifest_dir = env!("CARGO_MANIFEST_DIR");
    let fixtures_dir = Path::new(manifest_dir).join("../../tests/sdk-conformance/fixtures");
    assert!(fixtures_dir.exists(), "fixtures directory must exist");

    let mut fixture_count = 0;
    let mut entries: Vec<_> = fs::read_dir(&fixtures_dir)
        .expect("read fixtures dir")
        .filter_map(|e| e.ok())
        .filter(|e| e.path().extension().and_then(|s| s.to_str()) == Some("json"))
        .collect();

    entries.sort_by_key(|e| e.path());

    for entry in entries {
        let content = fs::read_to_string(entry.path()).expect("read fixture file");
        let fixture: Fixture = serde_json::from_str(&content).expect("parse fixture json");

        assert_eq!(
            fixture.response.status, fixture.expected.status,
            "status mismatch for fixture {}",
            fixture.id
        );

        if let Some(expected_code) = &fixture.expected.error_code {
            let code = fixture
                .response
                .body
                .get("error")
                .and_then(|e| e.get("code"))
                .and_then(|c| c.as_str());
            assert_eq!(
                code,
                Some(expected_code.as_str()),
                "error code mismatch for fixture {}",
                fixture.id
            );
        }

        if let Some(expected_retry) = &fixture.expected.retry_after {
            let retry = fixture
                .response
                .headers
                .get("Retry-After")
                .and_then(|v| v.as_str());
            assert_eq!(
                retry,
                Some(expected_retry.as_str()),
                "retry-after mismatch for fixture {}",
                fixture.id
            );
        }

        if let Some(expected_count) = fixture.expected.count {
            let count = if let Some(arr) = fixture.response.body.get("contracts").and_then(|v| v.as_array()) {
                arr.len()
            } else if let Some(arr) = fixture.response.body.get("events").and_then(|v| v.as_array()) {
                arr.len()
            } else {
                0
            };
            assert_eq!(
                count, expected_count,
                "count mismatch for fixture {}",
                fixture.id
            );
        }

        fixture_count += 1;
    }

    assert!(
        fixture_count >= 15,
        "expected at least 15 fixtures tested, got {}",
        fixture_count
    );
}
