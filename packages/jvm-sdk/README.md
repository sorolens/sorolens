# sorolens-jvm

Official JVM client for the [Sorolens](../../) API, for Kotlin, Java, Android and
Spring services. It is generated from [`docs/openapi.yaml`](../../docs/openapi.yaml)
where that is useful, with a small hand-written layer for the things generators
do not get right: retries, auto-pagination and webhook verification.

- Java 17+, Gradle build, no runtime dependency except Jackson.
- Transport is `java.net.http.HttpClient` (JDK built-in), so the jar stays thin.
- Safe requests retry on `429`, `500`, `502`, `503` and `504` with bounded
  exponential backoff and jitter, honouring `Retry-After`.
- `PageIterator` follows `next_cursor` until the last page.
- `Webhooks.verify` implements the `X-Sorolens-Signature` HMAC-SHA256 scheme with
  a five-minute replay window and constant-time comparison.

## Build and test

```bash
cd packages/jvm-sdk
./gradlew test
./gradlew jar
```

## Quickstart

```java
import xyz.sorolens.sdk.SorolensClient;

try (SorolensClient client = SorolensClient.builder()
        .apiKey("sl_live_...")
        .build()) {

    System.out.println(client.globalStats().path("tracked_contracts").asInt());

    // Auto-pagination: every contract, across every cursor page.
    for (var contract : client.contracts(100)) {
        System.out.println(contract.path("id").asText());
    }

    // Explicit single page.
    var page = client.listEvents("CDLZ...", null, 50);
    if (page.hasMore()) {
        var next = client.listEvents("CDLZ...", page.nextCursor(), 50);
    }
}
```

Override the API root for local or self-hosted deployments with
`.baseUrl("http://localhost:8080")`.

### Retries

```java
import java.time.Duration;
import xyz.sorolens.sdk.RetryPolicy;

SorolensClient client = SorolensClient.builder()
    .retryPolicy(new RetryPolicy(5, Duration.ofMillis(250), Duration.ofSeconds(20),
                                 RetryPolicy.DEFAULT_STATUSES))
    .build();
```

`RetryPolicy.none()` disables retries. Permanent responses (401, 403, other
4xx) surface as `SorolensException` immediately and are never retried; call
`statusCode()` and `isPermanent()` to branch on them.

### Webhook verification

```java
import xyz.sorolens.sdk.Webhooks;

boolean ok = Webhooks.verify(
    "whsec_...",                                     // subscription signing secret
    request.getHeader("X-Sorolens-Signature"),
    request.getHeader("X-Sorolens-Timestamp"),
    rawBodyBytes);                                   // the exact bytes, before JSON parsing

if (!ok) {
    response.setStatus(401);
}
```

The published test vector in [`docs/webhooks.md`](../../docs/webhooks.md) is part
of the test suite, so the implementation cannot drift from the documented
scheme.

## Generated models

The generator is pinned in [`openapi-generator-config.yaml`](openapi-generator-config.yaml)
and run by [`scripts/generate_jvm_client.sh`](scripts/generate_jvm_client.sh)
(OpenAPI Generator 7.10.0) in the existing `sdks` CI job. Generated sources are
**not committed**: only the config, the script and the hand-written facade are
reviewed, which keeps diff noise out of pull requests. Run the script locally to
inspect the generated surface.

## Publishing to Maven Central

Publishing is intentionally left to a maintainer. The Gradle `maven-publish`
plugin is configured with a `mavenJava` publication; wire it to the
`Sonatype`/`Central Portal` credentials and a signing key, then run:

```bash
./gradlew publish
```

Do not publish from CI until the group id and namespace are approved.

## Example

[`examples/WatchdogExample.java`](examples/WatchdogExample.java) walks every
tracked contract and a contract's events; compile it against the built jar as
shown at the top of the file.
