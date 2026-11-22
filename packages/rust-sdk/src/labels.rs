//! List, save, and resolve account labels.

use serde::{Deserialize, Serialize};

use crate::client::ClientConfig;
use crate::error::Error;
use crate::macros::endpoints;
use crate::request::RequestSpec;

/// Visibility of a label when saving it.
#[derive(Debug, Clone, Copy, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum CreateLabelScope {
    /// Visible to all callers.
    Public,
    /// Visible only to the owning workspace.
    Workspace,
}

/// JSON body for creating or updating a label.
#[derive(Debug, Clone, Serialize)]
pub struct CreateLabelRequest {
    /// Human-readable label.
    pub label: String,
    /// Stellar account or contract ID.
    pub value: String,
    /// Visibility of the saved label.
    pub scope: CreateLabelScope,
}

/// A label or resolved Stellar identifier returned by the API.
#[derive(Debug, Clone, Deserialize)]
pub struct AccountLabel {
    /// Human-readable label or the directly resolved Stellar ID.
    pub label: String,
    /// Stellar account or contract ID.
    pub value: String,
    /// `"public"`, `"workspace"`, or `"raw"` for a directly resolved ID.
    pub scope: String,
}

/// Visible labels returned by the API.
#[derive(Debug, Clone, Deserialize)]
pub struct LabelsResponse {
    /// Labels visible to the caller.
    pub labels: Vec<AccountLabel>,
}

pub(crate) fn list_labels_request(
    _config: &ClientConfig,
    query: Option<&str>,
) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get("/api/v1/labels").query_opt("query", query))
}

pub(crate) fn create_label_request(
    _config: &ClientConfig,
    request: &CreateLabelRequest,
) -> Result<RequestSpec, Error> {
    RequestSpec::post("/api/v1/labels", request)
}

pub(crate) fn resolve_label_request(
    _config: &ClientConfig,
    query: &str,
) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get("/api/v1/resolve").query_opt("query", Some(query)))
}

endpoints! {
    /// Lists visible labels, optionally filtered by a query.
    fn list_labels(query: Option<&str>) -> LabelsResponse = list_labels_request;

    /// Creates or updates a label. Requires contributor access.
    fn create_label(request: &CreateLabelRequest) -> AccountLabel = create_label_request;

    /// Resolves a label or Stellar identifier.
    fn resolve_label(query: &str) -> AccountLabel = resolve_label_request;
}
