//! Error types returned by the Sorolens SDK.

use std::fmt;

use serde::de::DeserializeOwned;

/// Errors returned by the Sorolens SDK.
///
/// Every fallible SDK operation returns [`Result<T, Error>`](std::result::Result).
#[derive(Debug, thiserror::Error)]
#[non_exhaustive]
pub enum Error {
    /// The request could not be sent, or the response body could not be read.
    #[error("transport error: {0}")]
    Transport(#[source] Box<dyn std::error::Error + Send + Sync>),

    /// A response body could not be deserialized, or a request body could not
    /// be serialized, as JSON.
    #[error("json error: {0}")]
    Json(#[from] serde_json::Error),

    /// The client was configured with an invalid value.
    #[error("invalid configuration: {0}")]
    Config(String),

    /// A path or query parameter failed local validation.
    #[error("invalid request: {0}")]
    InvalidRequest(String),

    /// The API returned a non-success status code.
    #[error("sorolens api error: {0}")]
    Api(ApiError),
}

impl Error {
    /// Wraps a transport-level error (DNS, TLS, connection, timeout, ...).
    pub(crate) fn transport<E>(err: E) -> Self
    where
        E: Into<Box<dyn std::error::Error + Send + Sync>>,
    {
        Error::Transport(err.into())
    }

    /// The HTTP status code, when the failure came from an API response.
    pub fn status(&self) -> Option<u16> {
        match self {
            Error::Api(err) => Some(err.status),
            _ => None,
        }
    }

    /// The structured API error, when the failure came from an API response.
    pub fn api_error(&self) -> Option<&ApiError> {
        match self {
            Error::Api(err) => Some(err),
            _ => None,
        }
    }
}

/// A structured error payload returned by the Sorolens API for non-2xx responses.
///
/// The API nests its payload under an `error` key, which is either a plain
/// string or an object carrying `code` / `message` / `request_id`.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub struct ApiError {
    /// HTTP status code of the failed response.
    pub status: u16,
    /// Human-readable message, synthesised from the response body when absent.
    pub message: String,
    /// Machine-readable error code such as `INVALID_INPUT`, when present.
    pub code: Option<String>,
    /// Server-side request id, useful when reporting a problem upstream.
    pub request_id: Option<String>,
}

impl fmt::Display for ApiError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "HTTP {}", self.status)?;
        if let Some(code) = &self.code {
            write!(f, " {code}")?;
        }
        if !self.message.is_empty() {
            write!(f, ": {}", self.message)?;
        }
        if let Some(request_id) = &self.request_id {
            write!(f, " (request_id: {request_id})")?;
        }
        Ok(())
    }
}

impl ApiError {
    /// Returns the error code, if the API supplied one.
    pub fn code(&self) -> Option<&str> {
        self.code.as_deref()
    }

    /// Returns the server-side request id, if the API supplied one.
    pub fn request_id(&self) -> Option<&str> {
        self.request_id.as_deref()
    }
}

/// Deserializes a successful response body, or converts a non-2xx status into
/// [`Error::Api`].
pub(crate) fn parse_json<T: DeserializeOwned>(status: u16, body: &[u8]) -> Result<T, Error> {
    if !(200..300).contains(&status) {
        return Err(Error::Api(parse_api_error(status, body)));
    }
    serde_json::from_slice(body).map_err(Error::Json)
}

/// Builds an [`ApiError`] from a failed response, tolerating every error shape
/// documented in `docs/openapi.yaml`.
pub(crate) fn parse_api_error(status: u16, body: &[u8]) -> ApiError {
    let mut err = ApiError {
        status,
        message: String::new(),
        code: None,
        request_id: None,
    };
    let mut required = None;

    if let Ok(value) = serde_json::from_slice::<serde_json::Value>(body) {
        if let Some(inner) = value.get("error") {
            match inner {
                serde_json::Value::String(message) => err.message = message.clone(),
                serde_json::Value::Object(map) => {
                    if let Some(code) = map.get("code").and_then(serde_json::Value::as_str) {
                        err.code = Some(code.to_string());
                    }
                    if let Some(message) = map.get("message").and_then(serde_json::Value::as_str) {
                        err.message = message.to_string();
                    }
                    if let Some(request_id) =
                        map.get("request_id").and_then(serde_json::Value::as_str)
                    {
                        err.request_id = Some(request_id.to_string());
                    }
                    // `ScopeError` and `RoleError` nest a plain string here.
                    if err.message.is_empty() {
                        if let Some(message) = map.get("error").and_then(serde_json::Value::as_str)
                        {
                            err.message = message.to_string();
                        }
                    }
                }
                _ => {}
            }
        }
        if err.message.is_empty() {
            if let Some(message) = value.get("message").and_then(serde_json::Value::as_str) {
                err.message = message.to_string();
            }
        }
        if err.code.is_none() {
            if let Some(code) = value.get("code").and_then(serde_json::Value::as_str) {
                err.code = Some(code.to_string());
            }
        }
        if err.request_id.is_none() {
            if let Some(request_id) = value.get("request_id").and_then(serde_json::Value::as_str) {
                err.request_id = Some(request_id.to_string());
            }
        }
        required = value
            .get("required")
            .and_then(serde_json::Value::as_str)
            .map(str::to_string);
    }

    if let Some(required) = required {
        if err.message.is_empty() {
            err.message = format!("missing required value: {required}");
        } else {
            err.message = format!("{} (required: {required})", err.message);
        }
    }

    if err.message.is_empty() {
        let text = String::from_utf8_lossy(body).trim().to_string();
        err.message = if text.is_empty() {
            format!("request failed with status {status}")
        } else {
            truncate(&text, 512)
        };
    }

    err
}

fn truncate(text: &str, max: usize) -> String {
    if text.len() <= max {
        return text.to_string();
    }
    let mut end = max;
    while !text.is_char_boundary(end) {
        end -= 1;
    }
    format!("{}...", &text[..end])
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_nested_error_object() {
        let err = parse_api_error(
            422,
            br#"{"error":{"code":"INVALID_INPUT","message":"bad cursor","request_id":"req_1"}}"#,
        );
        assert_eq!(err.status, 422);
        assert_eq!(err.message, "bad cursor");
        assert_eq!(err.code(), Some("INVALID_INPUT"));
        assert_eq!(err.request_id(), Some("req_1"));
    }

    #[test]
    fn parses_plain_string_error() {
        let err = parse_api_error(401, br#"{"error":"missing credential"}"#);
        assert_eq!(err.message, "missing credential");
        assert_eq!(err.code(), None);
    }

    #[test]
    fn parses_scope_error() {
        let err = parse_api_error(403, br#"{"error":"forbidden","required":"admin:*"}"#);
        assert_eq!(err.message, "forbidden (required: admin:*)");
    }

    #[test]
    fn falls_back_to_raw_body() {
        let err = parse_api_error(500, b"boom");
        assert_eq!(err.message, "boom");
    }

    #[test]
    fn api_error_display_includes_status_and_code() {
        let err = parse_api_error(
            429,
            br#"{"error":{"code":"RATE_LIMITED","message":"slow down"}}"#,
        );
        assert_eq!(err.to_string(), "HTTP 429 RATE_LIMITED: slow down");
    }

    #[test]
    fn parse_json_rejects_non_success() {
        let err = parse_json::<serde_json::Value>(404, br#"{"error":"nope"}"#).unwrap_err();
        assert_eq!(err.status(), Some(404));
        assert_eq!(err.api_error().map(ApiError::code), Some(None::<&str>),);
    }
}
