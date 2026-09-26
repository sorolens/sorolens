#!/usr/bin/env bash
# Runs `pulumi preview` for examples/aws with NO cloud account:
# - state goes to a throwaway local file backend in a temp directory;
# - AWS credentials are fake strings and the provider is told not to
#   validate them, look up the account, or query instance metadata, so it
#   makes no AWS API calls;
# - availability zones are given explicitly so nothing is looked up.
# preview never creates resources. Used by CI (works on fork PRs, no secrets).
set -euo pipefail

cd "$(dirname "$0")/../examples/aws"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

export PULUMI_BACKEND_URL="file://${work}/state"
# Encrypts secrets in the throwaway state only; not a real secret.
export PULUMI_CONFIG_PASSPHRASE="offline-preview-not-a-secret"
export PULUMI_SKIP_UPDATE_CHECK=true
export AWS_ACCESS_KEY_ID="offline-preview-fake-key"
export AWS_SECRET_ACCESS_KEY="offline-preview-fake-secret"
# Never pick up real credentials from the machine running this.
: > "${work}/empty"
export AWS_SHARED_CREDENTIALS_FILE="${work}/empty"
export AWS_CONFIG_FILE="${work}/empty"
unset AWS_PROFILE AWS_SESSION_TOKEN
export AWS_EC2_METADATA_DISABLED=true

mkdir -p "${work}/state"
config="${work}/Pulumi.offline.yaml"
stack="offline-preview"

pulumi stack init "$stack" --non-interactive >/dev/null
# stack init writes the passphrase salt next to Pulumi.yaml; keep the tree clean.
trap 'rm -rf "$work"; rm -f "Pulumi.${stack}.yaml"' EXIT

set_config() { pulumi config set --stack "$stack" --config-file "$config" "$@" >/dev/null; }
set_config aws:region us-east-1
set_config aws:skipCredentialsValidation true
set_config aws:skipRequestingAccountId true
set_config aws:skipMetadataApiCheck true
set_config aws:skipRegionValidation true
set_config --path 'availabilityZones[0]' us-east-1a
set_config --path 'availabilityZones[1]' us-east-1b
set_config apiImage ghcr.io/example/sorolens-api:0.1.0
set_config indexerImage ghcr.io/example/sorolens-indexer:0.1.0
set_config dashboardImage ghcr.io/example/sorolens-dashboard:0.1.0

scenario="${1:-http}"
case "$scenario" in
  http)
    set_config allowHttpOnly true
    ;;
  https)
    set_config certificateArn arn:aws:acm:us-east-1:123456789012:certificate/00000000-0000-0000-0000-000000000000
    set_config domainName sorolens.example.com
    set_config route53ZoneId Z0000000000000000000
    ;;
  *)
    echo "usage: $0 [http|https]" >&2
    exit 2
    ;;
esac

pulumi preview --stack "$stack" --config-file "$config" --non-interactive --diff=false
