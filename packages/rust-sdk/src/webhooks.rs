//! Verify incoming Sorolens webhook deliveries.
//!
//! Every delivery carries two headers:
//!
//! ```text
//! X-Sorolens-Timestamp: <unix seconds>
//! X-Sorolens-Signature: t=<unix seconds>,v1=<hex HMAC-SHA256>
//! ```
//!
//! The signed payload is the timestamp, a `.` separator and the exact request
//! body:
//!
//! ```text
//! signed_payload = "<timestamp>." + raw_body
//! signature      = hex(HMAC_SHA256(signing_secret, signed_payload))
//! ```
//!
//! The signing secret is the full `whsec_...` string, used as raw UTF-8 bytes.
//! Timestamps outside the documented five-minute window are rejected so a
//! captured delivery cannot be replayed. See `docs/webhooks.md` for the
//! protocol and the shared test vector.
//!
//! ```no_run
//! use sorolens_sdk::webhooks::verify;
//!
//! # fn handler(secret: &str, signature: &str, timestamp: &str, body: &[u8]) -> Result<(), Box<dyn std::error::Error>> {
//! let now = std::time::SystemTime::now()
//!     .duration_since(std::time::UNIX_EPOCH)?
//!     .as_secs() as i64;
//! // Pass 0 to use the documented five-minute replay window.
//! verify(secret, signature, timestamp, body, now, 0)?;
//! # Ok(())
//! # }
//! ```
//!
//! The crate has no crypto dependency, so this module carries a minimal
//! SHA-256/HMAC implementation. It is checked against FIPS 180-4 and RFC 4231
//! vectors by the test suite.

/// The header carrying the timestamp and one or more `v1` signatures.
pub const SIGNATURE_HEADER: &str = "X-Sorolens-Signature";
/// The standalone header carrying the signed timestamp.
pub const TIMESTAMP_HEADER: &str = "X-Sorolens-Timestamp";
/// The current signature version.
pub const SIGNATURE_SCHEME: &str = "v1";
/// The documented replay window, in seconds.
pub const DEFAULT_TOLERANCE_SECONDS: i64 = 300;

/// A delivery failed signature or replay verification.
///
/// Treat it as a `401`/`403`: discard the payload and do not process it.
#[derive(Debug, Clone, Copy, PartialEq, Eq, thiserror::Error)]
#[non_exhaustive]
pub enum WebhookError {
    /// The signature or timestamp header was missing or malformed.
    #[error("malformed signature header")]
    MalformedHeader,
    /// The signature header carried no `v1` signature.
    #[error("no v1 signature present")]
    NoSignature,
    /// The timestamp is outside the replay window (too old or too far ahead).
    #[error("timestamp outside the replay window")]
    TimestampOutsideWindow,
    /// No `v1` signature matched the expected MAC.
    #[error("signature mismatch")]
    SignatureMismatch,
}

/// Returns the exact byte string covered by a signature:
/// `"<timestamp>.<body>"`.
pub fn signed_payload(timestamp: i64, body: &[u8]) -> Vec<u8> {
    let mut payload = timestamp.to_string().into_bytes();
    payload.push(b'.');
    payload.extend_from_slice(body);
    payload
}

/// Returns the hex HMAC-SHA256 `v1` signature for a body and timestamp.
pub fn sign(secret: &str, timestamp: i64, body: &[u8]) -> String {
    to_hex(&hmac_sha256(
        secret.as_bytes(),
        &signed_payload(timestamp, body),
    ))
}

/// Renders the value of [`SIGNATURE_HEADER`]:
/// `"t=<timestamp>,v1=<signature>"`.
pub fn signature_header(timestamp: i64, signature: &str) -> String {
    format!("t={timestamp},{SIGNATURE_SCHEME}={signature}")
}

/// Verifies a delivery against a signing secret.
///
/// `now` is the receiver's current Unix time in seconds. `tolerance_seconds`
/// is the allowed absolute difference; `0` or less selects
/// [`DEFAULT_TOLERANCE_SECONDS`]. Every `v1` signature is compared in constant
/// time, so a receiver may accept either of two secrets during rotation.
pub fn verify(
    secret: &str,
    signature_header: &str,
    timestamp_header: &str,
    body: &[u8],
    now: i64,
    tolerance_seconds: i64,
) -> Result<(), WebhookError> {
    let (timestamp, signatures) = parse_signature_header(signature_header)?;
    if signatures.is_empty() {
        return Err(WebhookError::NoSignature);
    }

    let header_timestamp: i64 = timestamp_header
        .trim()
        .parse()
        .map_err(|_| WebhookError::MalformedHeader)?;
    if header_timestamp != timestamp {
        return Err(WebhookError::MalformedHeader);
    }

    let tolerance = if tolerance_seconds <= 0 {
        DEFAULT_TOLERANCE_SECONDS as u64
    } else {
        tolerance_seconds as u64
    };
    if timestamp.abs_diff(now) > tolerance {
        return Err(WebhookError::TimestampOutsideWindow);
    }

    let expected = hmac_sha256(secret.as_bytes(), &signed_payload(timestamp, body));
    for signature in signatures {
        let received = decode_hex(&signature).ok_or(WebhookError::SignatureMismatch)?;
        if constant_time_eq(&received, &expected) {
            return Ok(());
        }
    }
    Err(WebhookError::SignatureMismatch)
}

/// Parses a signature header into its timestamp and every `v1` signature.
fn parse_signature_header(header: &str) -> Result<(i64, Vec<String>), WebhookError> {
    let header = header.trim();
    if header.is_empty() {
        return Err(WebhookError::MalformedHeader);
    }

    let mut timestamp = None;
    let mut signatures = Vec::new();
    for piece in header.split(',') {
        let (key, value) = piece.split_once('=').ok_or(WebhookError::MalformedHeader)?;
        let key = key.trim();
        let value = value.trim();
        match key {
            "t" => {
                timestamp = Some(value.parse().map_err(|_| WebhookError::MalformedHeader)?);
            }
            SIGNATURE_SCHEME if !value.is_empty() => signatures.push(value.to_string()),
            _ => {}
        }
    }

    timestamp
        .map(|timestamp| (timestamp, signatures))
        .ok_or(WebhookError::MalformedHeader)
}

// ---- Minimal SHA-256 / HMAC-SHA256 -----------------------------------------

const SHA256_K: [u32; 64] = [
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
    0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
    0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
    0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
    0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
    0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
];

const SHA256_H: [u32; 8] = [
    0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19,
];

fn sha256(data: &[u8]) -> [u8; 32] {
    let bit_len = (data.len() as u64).wrapping_mul(8);

    let mut message = data.to_vec();
    message.push(0x80);
    while message.len() % 64 != 56 {
        message.push(0);
    }
    message.extend_from_slice(&bit_len.to_be_bytes());

    let mut hash = SHA256_H;
    for block in message.chunks_exact(64) {
        let mut schedule = [0u32; 64];
        for (index, word) in schedule.iter_mut().take(16).enumerate() {
            let offset = index * 4;
            *word = u32::from_be_bytes([
                block[offset],
                block[offset + 1],
                block[offset + 2],
                block[offset + 3],
            ]);
        }
        for index in 16..64 {
            let s0 = schedule[index - 15].rotate_right(7)
                ^ schedule[index - 15].rotate_right(18)
                ^ (schedule[index - 15] >> 3);
            let s1 = schedule[index - 2].rotate_right(17)
                ^ schedule[index - 2].rotate_right(19)
                ^ (schedule[index - 2] >> 10);
            schedule[index] = schedule[index - 16]
                .wrapping_add(s0)
                .wrapping_add(schedule[index - 7])
                .wrapping_add(s1);
        }

        let mut a = hash[0];
        let mut b = hash[1];
        let mut c = hash[2];
        let mut d = hash[3];
        let mut e = hash[4];
        let mut f = hash[5];
        let mut g = hash[6];
        let mut h = hash[7];

        for index in 0..64 {
            let s1 = e.rotate_right(6) ^ e.rotate_right(11) ^ e.rotate_right(25);
            let choose = (e & f) ^ ((!e) & g);
            let temp1 = h
                .wrapping_add(s1)
                .wrapping_add(choose)
                .wrapping_add(SHA256_K[index])
                .wrapping_add(schedule[index]);
            let s0 = a.rotate_right(2) ^ a.rotate_right(13) ^ a.rotate_right(22);
            let majority = (a & b) ^ (a & c) ^ (b & c);
            let temp2 = s0.wrapping_add(majority);

            h = g;
            g = f;
            f = e;
            e = d.wrapping_add(temp1);
            d = c;
            c = b;
            b = a;
            a = temp1.wrapping_add(temp2);
        }

        hash[0] = hash[0].wrapping_add(a);
        hash[1] = hash[1].wrapping_add(b);
        hash[2] = hash[2].wrapping_add(c);
        hash[3] = hash[3].wrapping_add(d);
        hash[4] = hash[4].wrapping_add(e);
        hash[5] = hash[5].wrapping_add(f);
        hash[6] = hash[6].wrapping_add(g);
        hash[7] = hash[7].wrapping_add(h);
    }

    let mut digest = [0u8; 32];
    for (index, word) in hash.iter().enumerate() {
        digest[index * 4..index * 4 + 4].copy_from_slice(&word.to_be_bytes());
    }
    digest
}

fn hmac_sha256(key: &[u8], message: &[u8]) -> [u8; 32] {
    let mut block = [0u8; 64];
    if key.len() > block.len() {
        block[..32].copy_from_slice(&sha256(key));
    } else {
        block[..key.len()].copy_from_slice(key);
    }

    let mut inner = Vec::with_capacity(block.len() + message.len());
    inner.extend(block.iter().map(|byte| byte ^ 0x36));
    inner.extend_from_slice(message);

    let inner_digest = sha256(&inner);

    let mut outer = Vec::with_capacity(block.len() + inner_digest.len());
    outer.extend(block.iter().map(|byte| byte ^ 0x5c));
    outer.extend_from_slice(&inner_digest);

    sha256(&outer)
}

fn to_hex(bytes: &[u8]) -> String {
    let mut out = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        out.push_str(&format!("{byte:02x}"));
    }
    out
}

fn decode_hex(hex: &str) -> Option<Vec<u8>> {
    if hex.len() % 2 != 0 {
        return None;
    }
    (0..hex.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(&hex[index..index + 2], 16).ok())
        .collect()
}

fn constant_time_eq(left: &[u8], right: &[u8]) -> bool {
    if left.len() != right.len() {
        return false;
    }
    left.iter()
        .zip(right)
        .fold(0u8, |acc, (l, r)| acc | (l ^ r))
        == 0
}

#[cfg(test)]
mod tests {
    use super::*;

    // Shared vector from docs/webhooks.md, reproduced by the Go and Python
    // SDKs.
    const SECRET: &str = "whsec_example_0123456789abcdef0123456789abcdef";
    const TIMESTAMP: i64 = 1700000000;
    const BODY: &[u8] = br#"{"event":"watchdog.alert","severity":"Critical"}"#;
    const SIGNATURE: &str = "97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615";

    fn header() -> String {
        signature_header(TIMESTAMP, SIGNATURE)
    }

    #[test]
    fn sign_matches_published_vector() {
        assert_eq!(sign(SECRET, TIMESTAMP, BODY), SIGNATURE);
    }

    #[test]
    fn verify_accepts_published_vector() {
        assert_eq!(
            verify(
                SECRET,
                &header(),
                &TIMESTAMP.to_string(),
                BODY,
                TIMESTAMP,
                0
            ),
            Ok(())
        );
    }

    #[test]
    fn verify_rejects_tampered_body() {
        assert_eq!(
            verify(
                SECRET,
                &header(),
                &TIMESTAMP.to_string(),
                br#"{"event":"watchdog.alert","severity":"Info"}"#,
                TIMESTAMP,
                0,
            ),
            Err(WebhookError::SignatureMismatch)
        );
    }

    #[test]
    fn verify_rejects_wrong_secret() {
        assert_eq!(
            verify(
                "whsec_other",
                &header(),
                &TIMESTAMP.to_string(),
                BODY,
                TIMESTAMP,
                0,
            ),
            Err(WebhookError::SignatureMismatch)
        );
    }

    #[test]
    fn verify_rejects_stale_and_future_timestamps() {
        for now in [
            TIMESTAMP + DEFAULT_TOLERANCE_SECONDS + 1,
            TIMESTAMP - DEFAULT_TOLERANCE_SECONDS - 1,
        ] {
            assert_eq!(
                verify(SECRET, &header(), &TIMESTAMP.to_string(), BODY, now, 0),
                Err(WebhookError::TimestampOutsideWindow)
            );
        }
    }

    #[test]
    fn verify_accepts_timestamp_at_the_boundary() {
        assert_eq!(
            verify(
                SECRET,
                &header(),
                &TIMESTAMP.to_string(),
                BODY,
                TIMESTAMP + DEFAULT_TOLERANCE_SECONDS,
                0,
            ),
            Ok(())
        );
    }

    #[test]
    fn verify_accepts_any_matching_v1_signature() {
        let header = format!(
            "t={TIMESTAMP},v1={},v1={}",
            sign("whsec_old", TIMESTAMP, BODY),
            sign("whsec_new", TIMESTAMP, BODY),
        );
        for secret in ["whsec_old", "whsec_new"] {
            assert_eq!(
                verify(secret, &header, &TIMESTAMP.to_string(), BODY, TIMESTAMP, 0),
                Ok(())
            );
        }
    }

    #[test]
    fn verify_rejects_malformed_headers() {
        let valid = sign(SECRET, TIMESTAMP, BODY);
        let cases = [
            (String::new(), "1700000000", WebhookError::MalformedHeader),
            (
                format!("v1={valid}"),
                "1700000000",
                WebhookError::MalformedHeader,
            ),
            (
                format!("t=abc,v1={valid}"),
                "1700000000",
                WebhookError::MalformedHeader,
            ),
            (
                "t1700000000".to_string(),
                "1700000000",
                WebhookError::MalformedHeader,
            ),
            (
                format!("t=1700000000,v1={valid}"),
                "",
                WebhookError::MalformedHeader,
            ),
            (
                format!("t=1700000000,v1={valid}"),
                "1700000001",
                WebhookError::MalformedHeader,
            ),
        ];
        for (signature, timestamp, want) in cases {
            assert_eq!(
                verify(SECRET, &signature, timestamp, BODY, TIMESTAMP, 0),
                Err(want)
            );
        }
    }

    #[test]
    fn sha256_matches_fips_vectors() {
        assert_eq!(
            to_hex(&sha256(b"abc")),
            "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
        );
        assert_eq!(
            to_hex(&sha256(b"")),
            "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
        );
    }

    #[test]
    fn hmac_matches_rfc4231_vectors() {
        assert_eq!(
            to_hex(&hmac_sha256(&[0x0b; 20], b"Hi There")),
            "b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7"
        );
        assert_eq!(
            to_hex(&hmac_sha256(b"Jefe", b"what do ya want for nothing?")),
            "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"
        );
    }
}
