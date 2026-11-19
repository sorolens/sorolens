package xyz.sorolens.sdk;

/** A non-2xx response from the Sorolens API. */
public class SorolensException extends RuntimeException {

    private final int statusCode;
    private final String body;

    public SorolensException(int statusCode, String message, String body) {
        super(message);
        this.statusCode = statusCode;
        this.body = body;
    }

    public int statusCode() {
        return statusCode;
    }

    public String body() {
        return body;
    }

    /** True when retrying the same request cannot succeed (401, 403, other 4xx). */
    public boolean isPermanent() {
        return statusCode >= 400 && statusCode < 500 && statusCode != 408 && statusCode != 429;
    }
}
