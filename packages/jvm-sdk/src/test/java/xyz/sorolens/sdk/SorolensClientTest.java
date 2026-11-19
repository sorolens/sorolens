package xyz.sorolens.sdk;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.fasterxml.jackson.databind.JsonNode;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;

class SorolensClientTest {

    private HttpServer server;

    private String start(Handler handler) throws IOException {
        server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.createContext("/", exchange -> handler.handle(exchange));
        server.start();
        return "http://127.0.0.1:" + server.getAddress().getPort();
    }

    @AfterEach
    void stop() {
        if (server != null) {
            server.stop(0);
        }
    }

    private static void respond(HttpExchange exchange, int status, String body, String... headers)
            throws IOException {
        for (int i = 0; i < headers.length; i += 2) {
            exchange.getResponseHeaders().add(headers[i], headers[i + 1]);
        }
        byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
        exchange.getResponseHeaders().add("Content-Type", "application/json");
        exchange.sendResponseHeaders(status, bytes.length);
        try (OutputStream out = exchange.getResponseBody()) {
            out.write(bytes);
        }
    }

    private static SorolensClient.Builder builder(String baseUrl) {
        return SorolensClient.builder()
                .baseUrl(baseUrl)
                .retryPolicy(new RetryPolicy(2, Duration.ofMillis(1), Duration.ofMillis(5), RetryPolicy.DEFAULT_STATUSES));
    }

    @Test
    void retriesRetryableResponsesAndHonoursRetryAfter() throws Exception {
        AtomicInteger requests = new AtomicInteger();
        String baseUrl = start(exchange -> {
            if (requests.incrementAndGet() <= 2) {
                respond(exchange, 429, "{\"error\":{\"code\":\"RATE_LIMITED\"}}", "Retry-After", "0");
            } else {
                respond(exchange, 200, "{\"tracked_contracts\":42}");
            }
        });

        try (SorolensClient client = builder(baseUrl).build()) {
            JsonNode stats = client.globalStats();
            assertEquals(42, stats.path("tracked_contracts").asInt());
        }
        assertEquals(3, requests.get());
    }

    @Test
    void permanentErrorsAreNotRetried() throws Exception {
        AtomicInteger requests = new AtomicInteger();
        String baseUrl = start(exchange -> {
            requests.incrementAndGet();
            respond(exchange, 401, "{\"error\":{\"code\":\"UNAUTHORIZED\",\"message\":\"bad key\"}}");
        });

        try (SorolensClient client = builder(baseUrl).apiKey("wrong").build()) {
            SorolensException error = assertThrows(SorolensException.class, client::globalStats);
            assertEquals(401, error.statusCode());
            assertTrue(error.isPermanent());
        }
        assertEquals(1, requests.get());
    }

    @Test
    void paginatorWalksEveryPage() throws Exception {
        String baseUrl = start(exchange -> {
            String query = exchange.getRequestURI().getQuery();
            if (query != null && query.contains("cursor=page-2")) {
                respond(exchange, 200, "{\"contracts\":[{\"id\":\"C3\"}],\"next_cursor\":\"\"}");
            } else {
                respond(exchange, 200, "{\"contracts\":[{\"id\":\"C1\"},{\"id\":\"C2\"}],\"next_cursor\":\"page-2\"}");
            }
        });

        try (SorolensClient client = builder(baseUrl).build()) {
            List<String> ids = new ArrayList<>();
            for (JsonNode contract : client.contracts(2)) {
                ids.add(contract.path("id").asText());
            }
            assertEquals(List.of("C1", "C2", "C3"), ids);
        }
    }

    @Test
    void pageWithoutNextCursorStopsImmediately() throws Exception {
        String baseUrl = start(exchange ->
                respond(exchange, 200, "{\"contracts\":[{\"id\":\"C1\"}],\"next_cursor\":null}"));

        try (SorolensClient client = builder(baseUrl).build()) {
            Page<JsonNode> page = client.listContracts(null, 10);
            assertFalse(page.hasMore());
            assertEquals(1, page.items().size());
        }
    }

    @Test
    void webhookVerificationMatchesThePublishedVector() {
        String secret = "whsec_example_0123456789abcdef0123456789abcdef";
        String body = "{\"event\":\"watchdog.alert\",\"severity\":\"Critical\"}";
        String signature =
                "t=1700000000,v1=97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615";

        assertTrue(Webhooks.verify(secret, signature, "1700000000",
                body.getBytes(StandardCharsets.UTF_8), 1700000000L, 300));
        assertFalse(Webhooks.verify(secret, signature, "1700000000",
                body.replace("Critical", "Info").getBytes(StandardCharsets.UTF_8), 1700000000L, 300));
        assertFalse(Webhooks.verify(secret, signature, "1700000000",
                body.getBytes(StandardCharsets.UTF_8), 1700001000L, 300));
        assertFalse(Webhooks.verify(secret, signature, "1700000001",
                body.getBytes(StandardCharsets.UTF_8), 1700000000L, 300));
    }

    @Test
    void retryAfterParsesSecondsAndDates() {
        assertEquals(Duration.ofSeconds(12), RetryPolicy.parseRetryAfter("12"));
        assertEquals(Duration.ZERO, RetryPolicy.parseRetryAfter("0"));
        assertEquals(null, RetryPolicy.parseRetryAfter("nonsense"));
    }

    @FunctionalInterface
    private interface Handler {
        void handle(HttpExchange exchange) throws IOException;
    }
}
