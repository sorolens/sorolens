//! Client configuration and the async (default) transport.

use std::time::Duration;

use crate::error::{parse_json, Error};

/// Base URL used when none is configured.
pub const DEFAULT_BASE_URL: &str = "https://api.sorolens.xyz";

/// Default `User-Agent` sent with every request.
pub const DEFAULT_USER_AGENT: &str = concat!("sorolens-sdk/", env!("CARGO_PKG_VERSION"));

/// Shared configuration for the async and blocking clients.
///
/// Build one with [`ClientConfig::builder`], then hand it to [`Client::new`] or
/// to `BlockingClient::new` when the `blocking` feature is enabled.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ClientConfig {
    pub(crate) base_url: String,
    pub(crate) api_key: Option<String>,
    pub(crate) user_id: Option<String>,
    pub(crate) user_agent: String,
    pub(crate) timeout: Option<Duration>,
}

impl Default for ClientConfig {
    fn default() -> Self {
        Self {
            base_url: DEFAULT_BASE_URL.to_string(),
            api_key: None,
            user_id: None,
            user_agent: DEFAULT_USER_AGENT.to_string(),
            timeout: None,
        }
    }
}

impl ClientConfig {
    /// Starts building a configuration.
    pub fn builder() -> ClientConfigBuilder {
        ClientConfigBuilder::new()
    }

    /// The base URL requests are sent to.
    pub fn base_url(&self) -> &str {
        &self.base_url
    }

    /// The API key sent as `Authorization: Bearer <token>`, if configured.
    pub fn api_key(&self) -> Option<&str> {
        self.api_key.as_deref()
    }

    /// The identity sent as `X-User-ID`, if configured.
    pub fn user_id(&self) -> Option<&str> {
        self.user_id.as_deref()
    }

    /// The `User-Agent` header value.
    pub fn user_agent(&self) -> &str {
        &self.user_agent
    }

    /// The per-request timeout, if configured.
    pub fn timeout(&self) -> Option<Duration> {
        self.timeout
    }

    /// Joins a request path onto the configured base URL.
    pub(crate) fn endpoint(&self, path: &str) -> String {
        let base = self.base_url.trim_end_matches('/');
        if path.starts_with('/') {
            format!("{base}{path}")
        } else {
            format!("{base}/{path}")
        }
    }
}

/// Builder for [`ClientConfig`].
#[derive(Debug, Clone, Default)]
pub struct ClientConfigBuilder {
    config: ClientConfig,
}

impl ClientConfigBuilder {
    /// Creates a builder holding the default configuration.
    pub fn new() -> Self {
        Self {
            config: ClientConfig::default(),
        }
    }

    /// Sets the base URL, for example `https://api.sorolens.xyz` or
    /// `http://localhost:8080`. A trailing slash is optional.
    pub fn base_url(mut self, base_url: impl Into<String>) -> Self {
        self.config.base_url = base_url.into();
        self
    }

    /// Sets the API key, sent on every request as a bearer token.
    pub fn api_key(mut self, api_key: impl Into<String>) -> Self {
        self.config.api_key = Some(api_key.into());
        self
    }

    /// Sets the identity used by the watchlist endpoints (`X-User-ID`).
    pub fn user_id(mut self, user_id: impl Into<String>) -> Self {
        self.config.user_id = Some(user_id.into());
        self
    }

    /// Overrides the `User-Agent` header.
    pub fn user_agent(mut self, user_agent: impl Into<String>) -> Self {
        self.config.user_agent = user_agent.into();
        self
    }

    /// Sets a timeout applied to the whole request (connect + response).
    pub fn timeout(mut self, timeout: Duration) -> Self {
        self.config.timeout = Some(timeout);
        self
    }

    /// Validates the configuration.
    pub fn build(self) -> Result<ClientConfig, Error> {
        let base_url = self
            .config
            .base_url
            .trim()
            .trim_end_matches('/')
            .to_string();
        if base_url.is_empty() {
            return Err(Error::Config("base_url must not be empty".into()));
        }
        if !(base_url.starts_with("http://") || base_url.starts_with("https://")) {
            return Err(Error::Config(format!(
                "base_url must start with http:// or https://, got {base_url:?}"
            )));
        }
        Ok(ClientConfig {
            base_url,
            ..self.config
        })
    }
}

/// Asynchronous Sorolens client.
///
/// Available when the default `async` feature is enabled. The client is cheap
/// to clone and is intended to be created once and shared.
///
/// # Examples
///
/// ```no_run
/// use sorolens_sdk::{Client, Error, ListContractsParams};
///
/// # async fn run() -> Result<(), Error> {
/// let client = Client::builder()
///     .base_url("https://api.sorolens.xyz")
///     .api_key("sl_your_api_key")
///     .timeout(std::time::Duration::from_secs(10))
///     .build()?;
///
/// let page = client.list_contracts(&ListContractsParams::default()).await?;
/// for contract in &page.contracts {
///     println!("{} {}", contract.id, contract.status);
/// }
/// # Ok(())
/// # }
/// ```
#[cfg(feature = "async")]
#[derive(Debug, Clone)]
pub struct Client {
    pub(crate) http: reqwest::Client,
    pub(crate) config: ClientConfig,
}

#[cfg(feature = "async")]
impl Client {
    /// Builds a client from a validated [`ClientConfig`].
    pub fn new(config: ClientConfig) -> Result<Self, Error> {
        let mut builder = reqwest::Client::builder().user_agent(config.user_agent.clone());
        if let Some(timeout) = config.timeout {
            builder = builder.timeout(timeout);
        }
        let http = builder.build().map_err(Error::transport)?;
        Ok(Self { http, config })
    }

    /// Starts building a client.
    pub fn builder() -> ClientBuilder {
        ClientBuilder::new()
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
    pub(crate) async fn send<T: serde::de::DeserializeOwned>(
        &self,
        spec: crate::request::RequestSpec,
    ) -> Result<T, Error> {
        let request = build_request(&self.http, &self.config, &spec)?;
        let response = request.send().await.map_err(Error::transport)?;
        let status = response.status().as_u16();
        let body = response.bytes().await.map_err(Error::transport)?;
        parse_json(status, &body)
    }
}

/// Converts a [`RequestSpec`](crate::request::RequestSpec) into a reqwest
/// request with the configured auth headers applied.
#[cfg(feature = "async")]
pub(crate) fn build_request(
    http: &reqwest::Client,
    config: &ClientConfig,
    spec: &crate::request::RequestSpec,
) -> Result<reqwest::RequestBuilder, Error> {
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

/// Builder for the asynchronous [`Client`].
#[cfg(feature = "async")]
#[derive(Debug, Clone, Default)]
pub struct ClientBuilder {
    config: ClientConfigBuilder,
}

#[cfg(feature = "async")]
impl ClientBuilder {
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
    pub fn build(self) -> Result<Client, Error> {
        Client::new(self.config.build()?)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_config_points_at_production() {
        let config = ClientConfig::default();
        assert_eq!(config.base_url(), DEFAULT_BASE_URL);
        assert!(config.api_key().is_none());
    }

    #[test]
    fn builder_trims_trailing_slash() {
        let config = ClientConfig::builder()
            .base_url("http://localhost:8080/")
            .build()
            .unwrap();
        assert_eq!(config.base_url(), "http://localhost:8080");
        assert_eq!(
            config.endpoint("/api/v1/contracts"),
            "http://localhost:8080/api/v1/contracts"
        );
    }

    #[test]
    fn builder_rejects_empty_and_schemeless_urls() {
        assert!(matches!(
            ClientConfig::builder().base_url("  ").build(),
            Err(Error::Config(_))
        ));
        assert!(matches!(
            ClientConfig::builder().base_url("api.sorolens.xyz").build(),
            Err(Error::Config(_))
        ));
    }

    #[test]
    fn endpoint_joins_relative_paths() {
        let config = ClientConfig::builder()
            .base_url("https://api.sorolens.xyz")
            .build()
            .unwrap();
        assert_eq!(config.endpoint("health"), "https://api.sorolens.xyz/health");
    }

    #[cfg(feature = "async")]
    #[test]
    fn client_builder_builds_a_client() {
        let client = Client::builder()
            .base_url("https://api.sorolens.xyz")
            .api_key("sl_test")
            .build()
            .unwrap();
        assert_eq!(
            client.config().endpoint("/health"),
            "https://api.sorolens.xyz/health"
        );
        assert_eq!(client.clone().into_config().api_key(), Some("sl_test"));
    }
}
