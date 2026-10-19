//! Blocking transport, enabled by the `blocking` feature.
//!
//! The blocking client mirrors the asynchronous client method-for-method but
//! executes requests on the calling thread. It is meant for CLIs and test
//! harnesses that are not built on an async runtime.
//!
//! ```no_run
//! use sorolens_sdk::{BlockingClient, Error, ListContractsParams};
//!
//! # fn main() -> Result<(), Error> {
//! let client = BlockingClient::builder()
//!     .base_url("https://api.sorolens.xyz")
//!     .api_key("sl_your_api_key")
//!     .build()?;
//!
//! let page = client.list_contracts(&ListContractsParams::default())?;
//! println!("{} contracts", page.contracts.len());
//! # Ok(())
//! # }
//! ```

use std::time::Duration;

use crate::client::{ClientConfig, ClientConfigBuilder};
use crate::error::{parse_json, Error};

/// Blocking Sorolens client.
///
/// Available when the `blocking` feature is enabled. Constructing it inside an
/// async runtime is not supported by the underlying reqwest blocking transport.
#[derive(Debug, Clone)]
pub struct BlockingClient {
    pub(crate) http: reqwest::blocking::Client,
    pub(crate) config: ClientConfig,
}

impl BlockingClient {
    /// Builds a client from a validated [`ClientConfig`].
    pub fn new(config: ClientConfig) -> Result<Self, Error> {
        let mut builder =
            reqwest::blocking::Client::builder().user_agent(config.user_agent.clone());
        if let Some(timeout) = config.timeout {
            builder = builder.timeout(timeout);
        }
        let http = builder.build().map_err(Error::transport)?;
        Ok(Self { http, config })
    }

    /// Starts building a client.
    pub fn builder() -> BlockingClientBuilder {
        BlockingClientBuilder::new()
    }

    /// The configuration this client was built with.
    pub fn config(&self) -> &ClientConfig {
        &self.config
    }

    /// Consumes the client and returns its configuration.
    pub fn into_config(self) -> ClientConfig {
        self.config
    }

    /// Sends a described request and deserializes the JSON response.
    pub(crate) fn send<T: serde::de::DeserializeOwned>(
        &self,
        spec: crate::request::RequestSpec,
    ) -> Result<T, Error> {
        let request = build_request(&self.http, &self.config, &spec)?;
        let response = request.send().map_err(Error::transport)?;
        let status = response.status().as_u16();
        let body = response.bytes().map_err(Error::transport)?;
        parse_json(status, &body)
    }
}

fn build_request(
    http: &reqwest::blocking::Client,
    config: &ClientConfig,
    spec: &crate::request::RequestSpec,
) -> Result<reqwest::blocking::RequestBuilder, Error> {
    use crate::request::HttpMethod;

    let url = config.endpoint(&spec.path);
    let method = match spec.method {
        HttpMethod::Get => reqwest::Method::GET,
        HttpMethod::Post => reqwest::Method::POST,
        HttpMethod::Delete => reqwest::Method::DELETE,
    };
    let mut builder = http.request(method, url);
    if !spec.query.is_empty() {
        builder = builder.query(&spec.query);
    }
    if let Some(api_key) = &config.api_key {
        builder = builder.bearer_auth(api_key);
    }
    if let Some(user_id) = &config.user_id {
        builder = builder.header("X-User-ID", user_id);
    }
    if let Some(body) = &spec.body {
        builder = builder.json(body);
    }
    Ok(builder)
}

/// Builder for the [`BlockingClient`].
#[derive(Debug, Clone, Default)]
pub struct BlockingClientBuilder {
    config: ClientConfigBuilder,
}

impl BlockingClientBuilder {
    /// Creates a builder holding the default configuration.
    pub fn new() -> Self {
        Self::default()
    }

    /// Sets the base URL.
    pub fn base_url(mut self, base_url: impl Into<String>) -> Self {
        self.config = self.config.base_url(base_url);
        self
    }

    /// Sets the API key.
    pub fn api_key(mut self, api_key: impl Into<String>) -> Self {
        self.config = self.config.api_key(api_key);
        self
    }

    /// Sets the identity used by the watchlist endpoints.
    pub fn user_id(mut self, user_id: impl Into<String>) -> Self {
        self.config = self.config.user_id(user_id);
        self
    }

    /// Overrides the `User-Agent` header.
    pub fn user_agent(mut self, user_agent: impl Into<String>) -> Self {
        self.config = self.config.user_agent(user_agent);
        self
    }

    /// Sets a request timeout.
    pub fn timeout(mut self, timeout: Duration) -> Self {
        self.config = self.config.timeout(timeout);
        self
    }

    /// Validates the configuration and builds the client.
    pub fn build(self) -> Result<BlockingClient, Error> {
        BlockingClient::new(self.config.build()?)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn blocking_builder_builds_a_client() {
        let client = BlockingClient::builder()
            .base_url("http://localhost:8080/")
            .api_key("sl_test")
            .build()
            .unwrap();
        assert_eq!(
            client.config().endpoint("/api/v1/contracts"),
            "http://localhost:8080/api/v1/contracts"
        );
        assert_eq!(client.into_config().api_key(), Some("sl_test"));
    }
}
