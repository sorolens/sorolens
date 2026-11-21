//! API key administration.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use crate::client::ClientConfig;
use crate::error::Error;
use crate::macros::endpoints;
use crate::request::{encode_path_segment, HttpMethod, RequestSpec};

/// Metadata returned for an API key.
#[derive(Debug, Clone, Deserialize)]
pub struct ApiKey {
    /// Identifier used by the API key endpoints.
    pub id: String,
    /// Human-readable name.
    pub name: String,
    /// Visible prefix of the token.
    pub key_prefix: String,
    /// Permissions granted to the key.
    pub scopes: Vec<String>,
    /// User the key acts as, if bound to one.
    pub user_id: Option<String>,
    /// When the key was created.
    pub created_at: DateTime<Utc>,
    /// When the key was last used.
    pub last_used_at: Option<DateTime<Utc>>,
    /// When the key was revoked.
    pub revoked_at: Option<DateTime<Utc>>,
    /// When the key was last rotated.
    pub rotated_at: Option<DateTime<Utc>>,
}

/// A page of API key metadata.
#[derive(Debug, Clone, Deserialize)]
pub struct ApiKeysPage {
    /// Keys returned on this page.
    pub api_keys: Vec<ApiKey>,
    /// Cursor for the next page, or an empty string at the end.
    pub next_cursor: String,
}

/// Request body for creating a key.
#[derive(Debug, Clone, Serialize)]
pub struct CreateApiKeyRequest {
    /// Human-readable name.
    pub name: String,
    /// Permissions to grant to the key.
    pub scopes: Vec<String>,
    /// Optional user the key should act as.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub user_id: Option<String>,
}

/// Metadata and the plaintext token returned when a key is created or rotated.
#[derive(Deserialize)]
pub struct ApiKeyWithToken {
    /// Metadata for the key.
    #[serde(flatten)]
    pub metadata: ApiKey,
    /// Plaintext token, returned only once.
    pub key: String,
}

fn key_path(id: &str) -> Result<String, Error> {
    let id = id.trim();
    if id.is_empty() || matches!(id, "." | "..") {
        return Err(Error::InvalidRequest("invalid API key ID".into()));
    }
    Ok(format!("/api/v1/api-keys/{}", encode_path_segment(id)))
}

pub(crate) fn list_request(
    _config: &ClientConfig,
    cursor: Option<&str>,
    limit: Option<u32>,
) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get("/api/v1/api-keys")
        .query_opt("cursor", cursor)
        .query_opt("limit", limit))
}

pub(crate) fn create_request(
    _config: &ClientConfig,
    request: &CreateApiKeyRequest,
) -> Result<RequestSpec, Error> {
    RequestSpec::post("/api/v1/api-keys", request)
}

pub(crate) fn get_request(_config: &ClientConfig, id: &str) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get(key_path(id)?))
}

pub(crate) fn revoke_request(_config: &ClientConfig, id: &str) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::delete(key_path(id)?))
}

pub(crate) fn rotate_request(_config: &ClientConfig, id: &str) -> Result<RequestSpec, Error> {
    Ok(RequestSpec {
        method: HttpMethod::Post,
        path: format!("{}/rotate", key_path(id)?),
        query: Vec::new(),
        body: None,
    })
}

endpoints! {
    /// Lists API key metadata.
    fn list_api_keys(cursor: Option<&str>, limit: Option<u32>) -> ApiKeysPage = list_request;

    /// Creates an API key and returns its plaintext token.
    fn create_api_key(request: &CreateApiKeyRequest) -> ApiKeyWithToken = create_request;

    /// Fetches metadata for an API key.
    fn get_api_key(id: &str) -> ApiKey = get_request;

    /// Revokes an API key.
    fn revoke_api_key(id: &str) -> () = revoke_request;

    /// Rotates an API key and returns its new plaintext token.
    fn rotate_api_key(id: &str) -> ApiKeyWithToken = rotate_request;
}
