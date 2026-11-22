package xyz.sorolens.sdk;

import java.time.Duration;
import java.util.Set;
import java.util.concurrent.ThreadLocalRandom;

/**
 * Bounded exponential backoff with jitter for retryable responses.
 *
 * <p>Only safe requests are retried (the client never retries a request with a
 * body), and only statuses in {@link #retryStatuses()} (429 and common 5xx by
 * default). A {@code Retry-After} header on the response takes precedence over
 * the computed delay, capped at {@link #maxBackoff()}.
 */
public record RetryPolicy(
        int maxRetries,
        Duration initialBackoff,
        Duration maxBackoff,
        Set<Integer> retryStatuses) {

    public static final Set<Integer> DEFAULT_STATUSES = Set.of(429, 500, 502, 503, 504);

    public RetryPolicy {
        if (maxRetries < 0) {
            throw new IllegalArgumentException("maxRetries must be >= 0");
        }
        retryStatuses = Set.copyOf(retryStatuses);
    }

    public static RetryPolicy defaults() {
        return new RetryPolicy(2, Duration.ofMillis(500), Duration.ofSeconds(30), DEFAULT_STATUSES);
    }

    public static RetryPolicy none() {
        return new RetryPolicy(0, Duration.ZERO, Duration.ZERO, DEFAULT_STATUSES);
    }

    /** Delay before retry number {@code attempt} (zero-based). */
    public Duration delay(int attempt, String retryAfterHeader) {
        Duration retryAfter = parseRetryAfter(retryAfterHeader);
        if (retryAfter != null) {
            return retryAfter.compareTo(maxBackoff) > 0 ? maxBackoff : retryAfter;
        }
        long base = initialBackoff.toMillis() * (1L << Math.min(attempt, 20));
        long capped = Math.min(base, maxBackoff.toMillis());
        // Full jitter: anywhere in [capped/2, capped].
        long half = capped / 2;
        long delay = half + ThreadLocalRandom.current().nextLong(Math.max(1, half + 1));
        return Duration.ofMillis(delay);
    }

    /** Parse delta-seconds or an RFC 1123 HTTP-date; {@code null} when absent. */
    public static Duration parseRetryAfter(String value) {
        if (value == null || value.isBlank()) {
            return null;
        }
        value = value.trim();
        try {
            long seconds = Long.parseLong(value);
            return Duration.ofSeconds(Math.max(0, seconds));
        } catch (NumberFormatException ignored) {
            try {
                var when = java.time.ZonedDateTime.parse(
                        value, java.time.format.DateTimeFormatter.RFC_1123_DATE_TIME);
                long millis = java.time.Duration.between(java.time.ZonedDateTime.now(), when).toMillis();
                return Duration.ofMillis(Math.max(0, millis));
            } catch (RuntimeException alsoIgnored) {
                return null;
            }
        }
    }
}
