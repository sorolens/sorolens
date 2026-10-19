//! Transport-agnostic description of an API request.
//!
//! The per-resource modules build a [`RequestSpec`]; the async and blocking
//! transports in [`crate::client`] and [`crate::blocking`] turn it into a real
//! HTTP call. Keeping the shape in one place means the two transports cannot
//! drift apart.

use serde::Serialize;

use crate::error::Error;

/// HTTP method used by a [`RequestSpec`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum HttpMethod {
    Get,
    Post,
    Delete,
}

/// A fully-described API request, ready to be handed to a transport.
#[derive(Debug, Clone, PartialEq)]
pub(crate) struct RequestSpec {
    pub(crate) method: HttpMethod,
    pub(crate) path: String,
    pub(crate) query: Vec<(String, String)>,
    pub(crate) body: Option<serde_json::Value>,
}

impl RequestSpec {
    /// Builds a `GET` request for `path`.
    pub(crate) fn get(path: impl Into<String>) -> Self {
        Self {
            method: HttpMethod::Get,
            path: path.into(),
            query: Vec::new(),
            body: None,
        }
    }

    /// Builds a `DELETE` request for `path`.
    #[allow(dead_code)]
    pub(crate) fn delete(path: impl Into<String>) -> Self {
        Self {
            method: HttpMethod::Delete,
            path: path.into(),
            query: Vec::new(),
            body: None,
        }
    }

    /// Builds a `POST` request for `path` with a JSON body.
    pub(crate) fn post<T: Serialize>(path: impl Into<String>, body: &T) -> Result<Self, Error> {
        Ok(Self {
            method: HttpMethod::Post,
            path: path.into(),
            query: Vec::new(),
            body: Some(serde_json::to_value(body)?),
        })
    }

    /// Adds `key=value` when `value` is `Some`.
    pub(crate) fn query_opt<T: ToString>(mut self, key: &str, value: Option<T>) -> Self {
        if let Some(value) = value {
            self.query.push((key.to_string(), value.to_string()));
        }
        self
    }
}

/// Percent-encodes a single path segment, leaving unreserved characters alone.
///
/// Contract IDs are already alphanumeric, but user-supplied identifiers such as
/// watchdog contract IDs are encoded defensively so a stray `/` cannot rewrite
/// the request path.
pub(crate) fn encode_path_segment(segment: &str) -> String {
    let mut out = String::with_capacity(segment.len());
    for byte in segment.bytes() {
        match byte {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'.' | b'_' | b'~' => {
                out.push(byte as char);
            }
            _ => out.push_str(&format!("%{byte:02X}")),
        }
    }
    out
}

/// Validates and encodes a contract ID for use in a URL path.
pub(crate) fn contract_id(id: &str) -> Result<String, Error> {
    let id = id.trim();
    if id.is_empty() {
        return Err(Error::InvalidRequest(
            "contract id must not be empty".into(),
        ));
    }
    Ok(encode_path_segment(id))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn query_opt_skips_none_and_keeps_order() {
        let spec = RequestSpec::get("/api/v1/contracts")
            .query_opt("cursor", Some("abc"))
            .query_opt("limit", None::<u32>)
            .query_opt("limit", Some(50u32));
        assert_eq!(
            spec.query,
            vec![
                ("cursor".to_string(), "abc".to_string()),
                ("limit".to_string(), "50".to_string()),
            ]
        );
    }

    #[test]
    fn post_serializes_the_body() {
        let spec = RequestSpec::post("/api/v1/contracts", &serde_json::json!({"a": 1})).unwrap();
        assert_eq!(spec.method, HttpMethod::Post);
        assert_eq!(spec.body.unwrap()["a"], 1);
    }

    #[test]
    fn path_segments_are_percent_encoded() {
        assert_eq!(encode_path_segment("CABC/def"), "CABC%2Fdef");
        assert_eq!(encode_path_segment("a-b.c_d~e"), "a-b.c_d~e");
    }

    #[test]
    fn empty_contract_id_is_rejected() {
        let err = contract_id("   ").unwrap_err();
        assert!(matches!(err, Error::InvalidRequest(_)));
    }
}
