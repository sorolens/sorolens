//! Verify a Sorolens webhook delivery with the published HMAC-SHA256 scheme
//! ([docs/webhooks.md](https://github.com/sorolens/sorolens/blob/main/docs/webhooks.md)).
//!
//! It runs entirely offline against the pinned test vector, then shows a
//! tampered body and a stale timestamp being rejected. Pin your own receiver
//! against the same vector.
//!
//! ```sh
//! cargo run --example webhooks
//! ```

use hmac::{Hmac, Mac};
use sha2::Sha256;

const SECRET: &str = "whsec_example_0123456789abcdef0123456789abcdef";
const TIMESTAMP: &str = "1700000000";
const BODY: &[u8] = br#"{"event":"watchdog.alert","severity":"Critical"}"#;
const SIGNATURE: &str = "97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615";
const TOLERANCE_SECONDS: i64 = 300;

fn main() {
    match run() {
        Ok(()) => {}
        Err(err) => {
            eprintln!("webhooks: {err}");
            std::process::exit(1);
        }
    }
}

fn run() -> Result<(), Box<dyn std::error::Error>> {
    let header = format!("t={TIMESTAMP},v1={SIGNATURE}");
    let signed_at = 1_700_000_000;

    verify(SECRET, &header, TIMESTAMP, BODY, signed_at)?;
    println!("pinned test vector verified");

    let tampered = br#"{"event":"watchdog.alert","severity":"Critical","extra":true}"#;
    if verify(SECRET, &header, TIMESTAMP, tampered, signed_at).is_ok() {
        return Err("tampered body was not rejected".into());
    }
    println!("tampered body rejected");

    if verify(SECRET, &header, TIMESTAMP, BODY, signed_at + 301).is_ok() {
        return Err("stale delivery was not rejected".into());
    }
    println!("stale delivery rejected");

    Ok(())
}

/// Verify a delivery against the subscription's signing secret. Hashes the raw
/// body, compares in constant time (`verify_slice`), and enforces the replay
/// window. Verify before parsing the JSON.
fn verify(
    secret: &str,
    signature_header: &str,
    timestamp_header: &str,
    body: &[u8],
    now: i64,
) -> Result<(), Box<dyn std::error::Error>> {
    let (timestamp, signatures) = parse_signature_header(signature_header)?;
    if signatures.is_empty() {
        return Err("no v1 signature present".into());
    }
    if timestamp_header.trim().parse::<i64>()? != timestamp {
        return Err("timestamp header does not match the signed timestamp".into());
    }
    if (now - timestamp).abs() > TOLERANCE_SECONDS {
        return Err("timestamp outside the replay window".into());
    }

    for signature in &signatures {
        let bytes = hex_decode(signature)?;
        let mut mac = Hmac::<Sha256>::new_from_slice(secret.as_bytes())?;
        mac.update(timestamp.to_string().as_bytes());
        mac.update(b".");
        mac.update(body);
        if mac.verify_slice(&bytes).is_ok() {
            return Ok(());
        }
    }
    Err("signature mismatch".into())
}

/// Parse `t=<unix>[,v1=<hex>...]`; order is not significant.
fn parse_signature_header(header: &str) -> Result<(i64, Vec<String>), Box<dyn std::error::Error>> {
    let mut timestamp = None;
    let mut signatures = Vec::new();
    for part in header.split(',') {
        let (key, value) = part.split_once('=').ok_or("malformed signature header")?;
        match key.trim() {
            "t" => timestamp = Some(value.trim().parse::<i64>()?),
            "v1" => {
                let value = value.trim();
                if !value.is_empty() {
                    signatures.push(value.to_string());
                }
            }
            _ => {}
        }
    }
    Ok((
        timestamp.ok_or("missing timestamp in signature header")?,
        signatures,
    ))
}

fn hex_decode(value: &str) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    let value = value.trim();
    if value.len() % 2 != 0 {
        return Err("odd-length hex signature".into());
    }
    (0..value.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&value[i..i + 2], 16).map_err(Into::into))
        .collect()
}
