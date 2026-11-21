#!/usr/bin/env bash
#
# Smoke check for the local observability profile (issue #843).
#
# Verifies that Prometheus and Grafana answer, that the provisioned datasource,
# dashboard and alert rules loaded, and that every configured scrape target is
# up. Start the stack first:
#
#   docker compose --profile observability up -d
#   make api            # in one shell
#   cd services/indexer && go run .   # in another
#   ./scripts/observability-smoke.sh
#
# Environment:
#   PROM_URL               default http://localhost:9090
#   GRAFANA_URL            default http://localhost:3001
#   GRAFANA_ADMIN_USER     default admin
#   GRAFANA_ADMIN_PASSWORD falls back to reading the generated secret from the
#                          grafana container
set -euo pipefail

PROM_URL="${PROM_URL:-http://localhost:9090}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3001}"
GRAFANA_ADMIN_USER="${GRAFANA_ADMIN_USER:-admin}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

ok() {
  echo "ok   $*"
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

need curl

# --- Prometheus ------------------------------------------------------------

curl -fsS --max-time 10 "$PROM_URL/-/ready" >/dev/null \
  || fail "Prometheus is not ready at $PROM_URL"
ok "Prometheus is ready at $PROM_URL"

rules_json="$(curl -fsS --max-time 10 "$PROM_URL/api/v1/rules")" \
  || fail "cannot read alert rules from $PROM_URL/api/v1/rules"
for group in sorolens-indexer sorolens-api; do
  printf '%s' "$rules_json" | grep -q "\"name\":\"$group\"" \
    || fail "alert rule group '$group' is not loaded"
done
for alert in SorolensIndexerLagging SorolensIndexerSeverelyLagging SorolensAPICacheHitRatioLow; do
  printf '%s' "$rules_json" | grep -q "\"name\":\"$alert\"" \
    || fail "alert rule '$alert' is not loaded"
done
if printf '%s' "$rules_json" | grep -q '"health":"err"'; then
  fail "an alert rule failed to evaluate; see $PROM_URL/rules"
fi
ok "alert rules loaded without evaluation errors"

targets_json="$(curl -fsS --max-time 10 "$PROM_URL/api/v1/targets?state=active")" \
  || fail "cannot read scrape targets from $PROM_URL/api/v1/targets"
up_count="$(printf '%s' "$targets_json" | grep -o '"health":"up"' | wc -l | tr -d ' ' || true)"
down_count="$(printf '%s' "$targets_json" | grep -o '"health":"down"' | wc -l | tr -d ' ' || true)"
[ "$up_count" -gt 0 ] || fail "no scrape targets are up"
if [ "$down_count" -ne 0 ]; then
  down_targets="$(printf '%s' "$targets_json" \
    | grep -o '"scrapeUrl":"[^"]*"[^}]*"health":"down"' \
    | grep -o '"scrapeUrl":"[^"]*"' \
    | sed 's/"scrapeUrl":"//; s/"$//' \
    | paste -sd ', ' - || true)"
  fail "$down_count scrape target(s) are down: ${down_targets:-unknown}; start the API and indexer and retry"
fi
ok "$up_count scrape target(s) up"

# --- Grafana ---------------------------------------------------------------

password="${GRAFANA_ADMIN_PASSWORD:-}"
if [ -z "$password" ]; then
  password="$(docker compose exec -T grafana cat /etc/grafana-secrets/admin-password 2>/dev/null | tr -d '\r\n' || true)"
fi
[ -n "$password" ] || fail "could not read the Grafana admin password; set GRAFANA_ADMIN_PASSWORD"

curl -fsS --max-time 10 "$GRAFANA_URL/api/health" >/dev/null \
  || fail "Grafana is not healthy at $GRAFANA_URL"
ok "Grafana is healthy at $GRAFANA_URL"

curl -fsS --max-time 10 -u "$GRAFANA_ADMIN_USER:$password" \
  "$GRAFANA_URL/api/datasources/uid/sorolens-prometheus" >/dev/null \
  || fail "the provisioned Prometheus datasource is missing (or the admin password is wrong)"
ok "Prometheus datasource is provisioned"

dashboards_json="$(curl -fsS --max-time 10 -u "$GRAFANA_ADMIN_USER:$password" \
  "$GRAFANA_URL/api/search?query=Sorolens" || true)"
printf '%s' "$dashboards_json" | grep -q '"uid":"sorolens-overview"' \
  || fail "the provisioned 'Sorolens Overview' dashboard is missing"
ok "Sorolens Overview dashboard is provisioned"

echo
echo "Observability stack is healthy."
echo "  Prometheus: $PROM_URL"
echo "  Grafana:    $GRAFANA_URL (user: $GRAFANA_ADMIN_USER)"
