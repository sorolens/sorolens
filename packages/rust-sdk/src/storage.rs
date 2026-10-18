//! Contract storage entries and historical snapshots.
//!
//! Covers the `Storage` and `Snapshots` tags of `docs/openapi.yaml`
//! (`/api/v1/contracts/{id}/storage` and `/api/v1/contracts/{id}/snapshot`).

use crate::client::ClientConfig;
use crate::error::Error;
use crate::macros::endpoints;
use crate::request::{contract_id, RequestSpec};
use crate::types::{
    ContractSnapshot, Durability, Network, StorageEntry, StoragePage, StorageStatus,
};

/// Filters for [`crate::Client::list_contract_storage`].
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ListContractStorageParams {
    /// Opaque cursor from a previous page.
    pub cursor: Option<String>,
    /// Page size (`1..=200`, default 50).
    pub limit: Option<u32>,
    /// Restrict to one network.
    pub network: Option<Network>,
    /// Storage durability filter.
    pub durability: Option<Durability>,
    /// Lifecycle status filter.
    pub status: Option<StorageStatus>,
}

impl ListContractStorageParams {
    /// Sets the pagination cursor.
    pub fn cursor(mut self, cursor: impl Into<String>) -> Self {
        self.cursor = Some(cursor.into());
        self
    }

    /// Sets the page size.
    pub fn limit(mut self, limit: u32) -> Self {
        self.limit = Some(limit);
        self
    }

    /// Restricts the page to one network.
    pub fn network(mut self, network: Network) -> Self {
        self.network = Some(network);
        self
    }

    /// Filters by storage durability.
    pub fn durability(mut self, durability: Durability) -> Self {
        self.durability = Some(durability);
        self
    }

    /// Filters by lifecycle status.
    pub fn status(mut self, status: StorageStatus) -> Self {
        self.status = Some(status);
        self
    }
}

/// Parameters for [`crate::Client::get_contract_snapshot`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct GetContractSnapshotParams {
    /// Ledger to snap; must be greater than zero.
    pub ledger: i32,
}

impl GetContractSnapshotParams {
    /// Creates snapshot parameters for `ledger`.
    pub fn new(ledger: i32) -> Self {
        Self { ledger }
    }
}

pub(crate) fn list_request(
    _config: &ClientConfig,
    id: &str,
    params: &ListContractStorageParams,
) -> Result<RequestSpec, Error> {
    Ok(
        RequestSpec::get(format!("/api/v1/contracts/{}/storage", contract_id(id)?))
            .query_opt("cursor", params.cursor.as_deref())
            .query_opt("limit", params.limit)
            .query_opt("network", params.network.map(Network::as_str))
            .query_opt("durability", params.durability.map(Durability::as_str))
            .query_opt("status", params.status.map(StorageStatus::as_str)),
    )
}

pub(crate) fn snapshot_request(
    _config: &ClientConfig,
    id: &str,
    params: &GetContractSnapshotParams,
) -> Result<RequestSpec, Error> {
    if params.ledger <= 0 {
        return Err(Error::InvalidRequest(
            "snapshot ledger must be greater than zero".into(),
        ));
    }
    Ok(
        RequestSpec::get(format!("/api/v1/contracts/{}/snapshot", contract_id(id)?))
            .query_opt("ledger", Some(params.ledger)),
    )
}

endpoints! {
    /// Lists storage entries for a contract.
    ///
    /// `GET /api/v1/contracts/{id}/storage`.
    fn list_contract_storage(
        id: &str,
        params: &ListContractStorageParams,
    ) -> StoragePage = list_request;

    /// Snapshots a contract's storage at a historical ledger.
    ///
    /// `GET /api/v1/contracts/{id}/snapshot`.
    fn get_contract_snapshot(
        id: &str,
        params: &GetContractSnapshotParams,
    ) -> ContractSnapshot = snapshot_request;
}

/// Convenience accessor mirroring the decoded `key` on a [`StorageEntry`].
impl StorageEntry {
    /// The decoded key, if the indexer could decode it.
    pub fn decoded_key(&self) -> Option<&serde_json::Value> {
        self.key_decoded.as_ref()
    }

    /// The decoded value, if the indexer could decode it.
    pub fn decoded_value(&self) -> Option<&serde_json::Value> {
        self.value_decoded.as_ref()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn list_request_encodes_durability_and_status() {
        let config = ClientConfig::default();
        let params = ListContractStorageParams::default()
            .durability(Durability::Persistent)
            .status(StorageStatus::Live);
        let spec = list_request(&config, "CABC", &params).unwrap();
        assert_eq!(spec.path, "/api/v1/contracts/CABC/storage");
        assert_eq!(
            spec.query,
            vec![
                ("durability".to_string(), "persistent".to_string()),
                ("status".to_string(), "live".to_string()),
            ]
        );
    }

    #[test]
    fn snapshot_request_requires_a_positive_ledger() {
        let config = ClientConfig::default();
        let err =
            snapshot_request(&config, "CABC", &GetContractSnapshotParams::new(0)).unwrap_err();
        assert!(matches!(err, Error::InvalidRequest(_)));

        let spec = snapshot_request(&config, "CABC", &GetContractSnapshotParams::new(42)).unwrap();
        assert_eq!(spec.path, "/api/v1/contracts/CABC/snapshot");
        assert_eq!(spec.query, vec![("ledger".to_string(), "42".to_string())]);
    }
}
