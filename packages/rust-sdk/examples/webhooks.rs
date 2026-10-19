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
//! The crate has no crypto dependency, so this example carries a minimal
//! SHA-256/HMAC implementation that self-checks against published vectors
//! (FIPS 180-4 and RFC 4231) before it is used. It runs offline:
//!
//! ```text
//! cargo run --example webhooks
//! ```

const TOLERANCE_SECONDS: i64 = 300;

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
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
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

fn sign(secret: &str, raw_body: &[u8], timestamp: i64) -> String {
    format!(
        "t={timestamp},v1={}",
        to_hex(&hmac_sha256(
            secret.as_bytes(),
            &signed_payload(timestamp, raw_body)
        ))
    )
}

fn verify(
    secret: &str,
    raw_body: &[u8],
    timestamp: &str,
    signature_header: &str,
    now: i64,
) -> Result<(), String> {
    let timestamp: i64 = timestamp
        .parse()
        .map_err(|_| "invalid X-Sorolens-Timestamp".to_string())?;

    let delta = now - timestamp;
    if delta.abs() > TOLERANCE_SECONDS {
        return Err(format!(
            "signature timestamp outside the tolerance window ({delta}s)"
        ));
    }

    let received = signature_header
        .split(',')
        .find_map(|piece| piece.strip_prefix("v1="))
        .ok_or_else(|| "signature header is missing the v1 value".to_string())?;

    let received =
        decode_hex(received).ok_or_else(|| "signature header is not valid hex".to_string())?;
    let expected = hmac_sha256(secret.as_bytes(), &signed_payload(timestamp, raw_body));

    if !constant_time_eq(&received, &expected) {
        return Err("signature mismatch".to_string());
    }
    Ok(())
}

fn signed_payload(timestamp: i64, raw_body: &[u8]) -> Vec<u8> {
    let mut payload = timestamp.to_string().into_bytes();
    payload.push(b'.');
    payload.extend_from_slice(raw_body);
    payload
}

fn self_check() -> Result<(), String> {
    let sha_vectors: [(&[u8], &str, &str); 3] = [
        (
            b"abc",
            "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
            "FIPS 180-4 \"abc\"",
        ),
        (
            b"",
            "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
            "FIPS 180-4 empty input",
        ),
        (
            b"abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq",
            "248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1",
            "FIPS 180-4 two-block message",
        ),
    ];

    for (input, expected, label) in sha_vectors.iter() {
        let digest = to_hex(&sha256(input));
        if digest != *expected {
            return Err(format!("{label}: got {digest}"));
        }
    }

    let hmac_vectors: [(&[u8], &[u8], &str, &str); 3] = [
        (
            &[0x0b; 20],
            b"Hi There",
            "b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7",
            "RFC 4231 case 1",
        ),
        (
            b"Jefe",
            b"what do ya want for nothing?",
            "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843",
            "RFC 4231 case 2",
        ),
        (
            &[0xaa; 20],
            b"dddddddddddddddddddddddddddddddddddddddddddddddddd",
            "8ef30fb3b4fce8b883e34e8d9d444851a893f5065c6c0b8cd908962c45bc2b92",
            "RFC 4231 case 3",
        ),
    ];

    for (key, message, expected, label) in hmac_vectors.iter() {
        let digest = to_hex(&hmac_sha256(key, message));
        if digest != *expected {
            return Err(format!("{label}: got {digest}"));
        }
    }

    let long_key = [0xaa; 131];
    let digest = to_hex(&hmac_sha256(
        &long_key,
        b"Test Using Larger Than Block-Size Key - Hash Key First",
    ));
    let expected = "60e431591ee0b67f0d8a26aacbf5b77f8e0bc6213728c5140546040f0ee37f54";
    if digest != expected {
        return Err(format!("RFC 4231 long-key HMAC vector: got {digest}"));
    }
    Ok(())
}

fn expect_rejection(label: &str, result: Result<(), String>) {
    match result {
        Err(message) => println!("  rejected ({label}): {message}"),
        Ok(()) => {
            eprintln!("rejected ({label}): signature should have been rejected");
            std::process::exit(1);
        }
    }
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    self_check()?;

    let secret = std::env::var("SOROLENS_WEBHOOK_SECRET")
        .unwrap_or_else(|_| "whsec_example_secret".to_string());
    let body = br#"{"event":"contract.alert","contract_id":"CDEMO"}"#;
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)?
        .as_secs() as i64;
    let header = sign(&secret, body, now);

    verify(&secret, body, &now.to_string(), &header, now)?;
    println!("  accepted: {}...", &header[..24]);

    expect_rejection(
        "tampered body",
        verify(
            &secret,
            br#"{"event":"evil"}"#,
            &now.to_string(),
            &header,
            now,
        ),
    );
    expect_rejection(
        "wrong secret",
        verify("whsec_other", body, &now.to_string(), &header, now),
    );

    let stale = now - 600;
    expect_rejection(
        "replayed delivery",
        verify(
            &secret,
            body,
            &stale.to_string(),
            &sign(&secret, body, stale),
            now,
        ),
    );

    println!("webhook signature verification passed");
    Ok(())
}
