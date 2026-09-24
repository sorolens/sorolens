with open("apps/api/internal/soroban/client.go", "r") as f:
    c = f.read()
c = c.replace('httpClient: &http.Client{},', 'httpClient: &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},')
if '"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"' not in c:
    c = c.replace('"net/http"', '"net/http"\n\t"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"')
with open("apps/api/internal/soroban/client.go", "w") as f:
    f.write(c)
