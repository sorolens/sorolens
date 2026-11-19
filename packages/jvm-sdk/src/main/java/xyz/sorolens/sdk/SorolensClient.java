package xyz.sorolens.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Synchronous JVM client for the Sorolens API.
 *
 * <p>The client is immutable apart from its lazily created {@link HttpClient}.
 * Safe requests (GET) are retried on 429/5xx with bounded exponential backoff
 * and jitter, honouring {@code Retry-After}; permanent failures are surfaced as
 * a {@link SorolensException} without retrying.
 *
 * <pre>{@code
 * try (var client = SorolensClient.builder().apiKey("sl_...").build()) {
 *     for (JsonNode contract : client.contracts(100)) {
 *         System.out.println(contract.path("id").asText());
 *     }
 * }
 * }</pre>
 */
public final class SorolensClient implements AutoCloseable {

    public static final String DEFAULT_BASE_URL = "https://api.sorolens.xyz";

    private final URI baseUri;
    private final String apiKey;
    private final ObjectMapper mapper;
    private final RetryPolicy retryPolicy;
    private final HttpClient http;
    private final Duration requestTimeout;

    private SorolensClient(Builder builder) {
        this.baseUri = URI.create(builder.baseUrl.replaceAll("/+$", ""));
        this.apiKey = builder.apiKey;
        this.mapper = new ObjectMapper();
        this.retryPolicy = builder.retryPolicy;
        this.requestTimeout = builder.requestTimeout;
        this.http = builder.httpClient != null
                ? builder.httpClient
                : HttpClient.newBuilder().connectTimeout(builder.connectTimeout).build();
    }

    public static Builder builder() {
        return new Builder();
    }

    /** Operation-named alias for {@link #globalStats()}. */
    public JsonNode getGlobalStats() {
        return globalStats();
    }

    public JsonNode globalStats() {
        return request("GET", "/api/v1/stats/global", Map.of());
    }

    public JsonNode getContract(String contractId) {
        return request("GET", "/api/v1/contracts/" + encodePath(contractId), Map.of());
    }

    public JsonNode getContractStats(String contractId, String window) {
        return request("GET", "/api/v1/contracts/" + encodePath(contractId) + "/stats", query(window));
    }

    public Page<JsonNode> listContracts(String cursor, Integer limit) {
        Map<String, String> query = new LinkedHashMap<>();
        put(query, "cursor", cursor);
        put(query, "limit", limit);
        JsonNode body = request("GET", "/api/v1/contracts", query);
        return page(body, "contracts");
    }

    public Page<JsonNode> listEvents(String contractId, String cursor, Integer limit) {
        Map<String, String> query = new LinkedHashMap<>();
        put(query, "cursor", cursor);
        put(query, "limit", limit);
        JsonNode body = request(
                "GET", "/api/v1/contracts/" + encodePath(contractId) + "/events", query);
        return page(body, "events");
    }

    public Page<JsonNode> listInvocations(String contractId, String cursor, Integer limit) {
        Map<String, String> query = new LinkedHashMap<>();
        put(query, "cursor", cursor);
        put(query, "limit", limit);
        JsonNode body = request(
                "GET", "/api/v1/contracts/" + encodePath(contractId) + "/invocations", query);
        return page(body, "invocations");
    }

    /** Auto-paginating iterator over every tracked contract. */
    public Iterable<JsonNode> contracts(Integer limit) {
        return new PageIterator<>(cursor -> listContracts(cursor, limit));
    }

    /** Operation-named alias for {@link #listEvents(String, String, Integer)}. */
    public Page<JsonNode> listContractEvents(String contractId, String cursor, Integer limit) {
        return listEvents(contractId, cursor, limit);
    }

    /** Operation-named alias for {@link #listInvocations(String, String, Integer)}. */
    public Page<JsonNode> listContractInvocations(
            String contractId, String cursor, Integer limit) {
        return listInvocations(contractId, cursor, limit);
    }

    /** Auto-paginating iterator over a contract's events. */
    public Iterable<JsonNode> events(String contractId, Integer limit) {
        return new PageIterator<>(cursor -> listEvents(contractId, cursor, limit));
    }

    /** Auto-paginating iterator over a contract's invocations. */
    public Iterable<JsonNode> invocations(String contractId, Integer limit) {
        return new PageIterator<>(cursor -> listInvocations(contractId, cursor, limit));
    }

    /**
     * Issue a request and return the parsed JSON body.
     *
     * <p>GET requests are retried according to the {@link RetryPolicy}; the
     * response of the final attempt is returned or raised.
     */
    public JsonNode request(String method, String path, Map<String, String> query) {
        URI uri = buildUri(path, query);
        boolean retryable = "GET".equalsIgnoreCase(method);
        int attempt = 0;
        while (true) {
            HttpResponse<String> response = send(method, uri);
            int status = response.statusCode();
            boolean retryStatus = retryPolicy.retryStatuses().contains(status);
            if (!retryable || !retryStatus || attempt >= retryPolicy.maxRetries()) {
                return handle(response);
            }
            Duration delay = retryPolicy.delay(attempt, response.headers().firstValue("Retry-After").orElse(null));
            attempt++;
            try {
                Thread.sleep(delay.toMillis());
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw new SorolensException(0, "interrupted while retrying", "");
            }
        }
    }

    private HttpResponse<String> send(String method, URI uri) {
        HttpRequest.Builder builder = HttpRequest.newBuilder(uri)
                .timeout(requestTimeout)
                .header("Accept", "application/json");
        if (apiKey != null && !apiKey.isBlank()) {
            builder.header("Authorization", "Bearer " + apiKey);
        }
        HttpRequest request = "GET".equalsIgnoreCase(method)
                ? builder.GET().build()
                : builder.method(method, HttpRequest.BodyPublishers.noBody()).build();
        try {
            return http.send(request, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
        } catch (IOException e) {
            throw new SorolensException(0, "request failed: " + e.getMessage(), "");
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new SorolensException(0, "request interrupted", "");
        }
    }

    private JsonNode handle(HttpResponse<String> response) {
        int status = response.statusCode();
        String body = response.body() == null ? "" : response.body();
        if (status < 200 || status > 299) {
            throw new SorolensException(status, errorMessage(status, body), body);
        }
        try {
            return body.isBlank() ? mapper.createObjectNode() : mapper.readTree(body);
        } catch (IOException e) {
            throw new SorolensException(status, "invalid JSON response: " + e.getMessage(), body);
        }
    }

    private String errorMessage(int status, String body) {
        try {
            JsonNode error = mapper.readTree(body).path("error");
            if (error.isObject()) {
                String message = error.path("message").asText("");
                String code = error.path("code").asText("");
                if (!message.isBlank()) {
                    return code.isBlank() ? message : code + ": " + message;
                }
            } else if (error.isTextual() && !error.asText().isBlank()) {
                return error.asText();
            }
        } catch (IOException ignored) {
            // Fall through to the status-only message.
        }
        return "HTTP " + status;
    }

    private Page<JsonNode> page(JsonNode body, String key) {
        List<JsonNode> items = new ArrayList<>();
        JsonNode array = body.path(key);
        if (array.isArray()) {
            array.forEach(items::add);
        }
        JsonNode next = body.path("next_cursor");
        String cursor = next.isTextual() ? next.asText() : null;
        return new Page<>(items, cursor);
    }

    private URI buildUri(String path, Map<String, String> query) {
        StringBuilder url = new StringBuilder(baseUri.toString()).append(path);
        if (!query.isEmpty()) {
            url.append('?');
            boolean first = true;
            for (Map.Entry<String, String> entry : query.entrySet()) {
                if (!first) {
                    url.append('&');
                }
                first = false;
                url.append(URLEncoder.encode(entry.getKey(), StandardCharsets.UTF_8))
                        .append('=')
                        .append(URLEncoder.encode(entry.getValue(), StandardCharsets.UTF_8));
            }
        }
        return URI.create(url.toString());
    }

    private static Map<String, String> query(String value) {
        Map<String, String> query = new LinkedHashMap<>();
        put(query, "window", value);
        return query;
    }

    private static void put(Map<String, String> query, String key, Object value) {
        if (value != null) {
            query.put(key, String.valueOf(value));
        }
    }

    private static String encodePath(String value) {
        return URLEncoder.encode(value, StandardCharsets.UTF_8).replace("+", "%20");
    }

    @Override
    public void close() {
        // java.net.http.HttpClient needs no explicit shutdown; the method keeps
        // try-with-resources usable and future-proofs a pooled implementation.
    }

    /** Builder for {@link SorolensClient}. */
    public static final class Builder {
        private String baseUrl = DEFAULT_BASE_URL;
        private String apiKey;
        private RetryPolicy retryPolicy = RetryPolicy.defaults();
        private Duration connectTimeout = Duration.ofSeconds(10);
        private Duration requestTimeout = Duration.ofSeconds(30);
        private HttpClient httpClient;

        public Builder baseUrl(String baseUrl) {
            this.baseUrl = baseUrl;
            return this;
        }

        public Builder apiKey(String apiKey) {
            this.apiKey = apiKey;
            return this;
        }

        public Builder retryPolicy(RetryPolicy retryPolicy) {
            this.retryPolicy = retryPolicy;
            return this;
        }

        public Builder requestTimeout(Duration requestTimeout) {
            this.requestTimeout = requestTimeout;
            return this;
        }

        public Builder connectTimeout(Duration connectTimeout) {
            this.connectTimeout = connectTimeout;
            return this;
        }

        /** Inject a pre-configured client, for tests or custom proxies. */
        public Builder httpClient(HttpClient httpClient) {
            this.httpClient = httpClient;
            return this;
        }

        public SorolensClient build() {
            return new SorolensClient(this);
        }
    }
}
