//! Verify the HMAC-SHA256 signature on incoming Sorolens webhook deliveries.
//!
//! Wire format (see `docs/webhooks.md`):
//!
//! ```text
//! X-Sorolens-Timestamp: <unix seconds>
//! X-Sorolens-Signature: t=<unix seconds>,v1=<hex HMAC-SHA256>
//!
//! signed_payload = "<timestamp>." + raw_body
//! signature      = hex(HMAC_SHA256(signing_secret, signed_payload))
//! ```
//!
//! The signing secret is the raw `whsec_...` string. Timestamps outside a
//! ±5 minute window are rejected to stop replayed deliveries.
//!
//! This example signs a delivery and verifies it end to end with
//! [`sorolens_sdk::webhooks`], so it runs offline:
//!
//! ```text
//! cargo run --example webhooks
//! ```

use sorolens_sdk::webhooks::{sign, signature_header, verify, WebhookError};

fn expect_rejection(label: &str, result: Result<(), WebhookError>) {
    match result {
        Err(error) => println!("  rejected ({label}): {error}"),
        Ok(()) => {
            eprintln!("rejected ({label}): signature should have been rejected");
            std::process::exit(1);
        }
    }
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let secret = std::env::var("SOROLENS_WEBHOOK_SECRET")
        .unwrap_or_else(|_| "whsec_example_secret".to_string());
    let body = br#"{"event":"contract.alert","contract_id":"CDEMO"}"#;
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)?
        .as_secs() as i64;
    let timestamp = now.to_string();
    let header = signature_header(now, &sign(&secret, now, body));

    verify(&secret, &header, &timestamp, body, now, 0)?;
    println!("  accepted: {}...", &header[..24]);

    expect_rejection(
        "tampered body",
        verify(&secret, &header, &timestamp, br#"{"event":"evil"}"#, now, 0),
    );
    expect_rejection(
        "wrong secret",
        verify("whsec_other", &header, &timestamp, body, now, 0),
    );

    let stale = now - 600;
    expect_rejection(
        "replayed delivery",
        verify(
            &secret,
            &signature_header(stale, &sign(&secret, stale, body)),
            &stale.to_string(),
            body,
            now,
            0,
        ),
    );

    println!("webhook signature verification passed");
    Ok(())
}
