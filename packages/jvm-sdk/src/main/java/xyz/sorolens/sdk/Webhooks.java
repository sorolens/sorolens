package xyz.sorolens.sdk;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.List;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

/**
 * Verification of {@code X-Sorolens-Signature} webhook deliveries.
 *
 * <p>The scheme is documented in {@code docs/webhooks.md}: the signed payload is
 * {@code timestamp + "." + rawBody}, HMAC-SHA256 keyed with the whole
 * {@code whsec_...} secret, hex encoded. Verification checks that both headers
 * agree, enforces a replay window, accepts any of the {@code v1} signatures and
 * compares in constant time.
 */
public final class Webhooks {

    public static final long DEFAULT_TOLERANCE_SECONDS = 300;

    private Webhooks() {}

    public static boolean verify(
            String secret, String signatureHeader, String timestampHeader, byte[] rawBody) {
        return verify(
                secret,
                signatureHeader,
                timestampHeader,
                rawBody,
                Instant.now().getEpochSecond(),
                DEFAULT_TOLERANCE_SECONDS);
    }

    public static boolean verify(
            String secret,
            String signatureHeader,
            String timestampHeader,
            byte[] rawBody,
            long nowEpochSeconds,
            long toleranceSeconds) {
        if (secret == null || signatureHeader == null || timestampHeader == null) {
            return false;
        }
        String timestamp = null;
        List<String> signatures = new ArrayList<>();
        for (String part : signatureHeader.split(",")) {
            int eq = part.indexOf('=');
            if (eq <= 0) {
                continue;
            }
            String key = part.substring(0, eq).trim();
            String value = part.substring(eq + 1).trim();
            if ("t".equals(key)) {
                timestamp = value;
            } else if ("v1".equals(key) && !value.isEmpty()) {
                signatures.add(value);
            }
        }
        if (timestamp == null || signatures.isEmpty() || !timestamp.equals(timestampHeader.trim())) {
            return false;
        }
        long ts;
        try {
            ts = Long.parseLong(timestamp);
        } catch (NumberFormatException e) {
            return false;
        }
        if (Math.abs(nowEpochSeconds - ts) > toleranceSeconds) {
            return false;
        }

        byte[] prefix = (timestamp + ".").getBytes(StandardCharsets.US_ASCII);
        byte[] body = rawBody == null ? new byte[0] : rawBody;
        byte[] payload = new byte[prefix.length + body.length];
        System.arraycopy(prefix, 0, payload, 0, prefix.length);
        System.arraycopy(body, 0, payload, prefix.length, body.length);
        // Hash the timestamp prefix and the body in one HMAC operation.
        byte[] actual = hmac(secret, payload);

        boolean matched = false;
        for (String signature : signatures) {
            byte[] candidate;
            try {
                candidate = HexFormat.of().parseHex(signature);
            } catch (IllegalArgumentException e) {
                continue;
            }
            // Do not short-circuit: compare every signature in constant time.
            matched |= MessageDigest.isEqual(candidate, actual);
        }
        return matched;
    }

    private static byte[] hmac(String secret, byte[] payload) {
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
            return mac.doFinal(payload);
        } catch (Exception e) {
            throw new IllegalStateException("HmacSHA256 is unavailable", e);
        }
    }
}
