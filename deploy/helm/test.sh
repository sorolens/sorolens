#!/usr/bin/env bash
# Checks the Sorolens Helm chart without a cluster:
#   1. helm lint with every scenario in ci/,
#   2. helm template for every scenario, validated by kubeconform against the
#      Kubernetes API schemas,
#   3. values the schema or the templates must refuse are refused.
#
# Usage: deploy/helm/test.sh   (needs helm and kubeconform on PATH)
# KUBERNETES_VERSION picks the schemas kubeconform validates against.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
chart="$here/sorolens"
k8s_version="${KUBERNETES_VERSION:-1.31.0}"
status=0

for values in "$here"/ci/*-values.yaml; do
  scenario="$(basename "$values" -values.yaml)"
  echo "::group::$scenario"
  helm lint --strict "$chart" -f "$values" --kube-version "$k8s_version"
  helm template sorolens "$chart" -f "$values" --namespace sorolens --kube-version "$k8s_version" \
    | kubeconform -strict -summary -kubernetes-version "$k8s_version"
  echo "::endgroup::"
done

# Each case is a set of --set flags and a message the failure must contain.
refuse() {
  local want="$1"
  shift
  local out
  if out="$(helm template sorolens "$chart" --kube-version "$k8s_version" -f "$here/ci/default-values.yaml" "$@" 2>&1)"; then
    echo "FAIL: expected '$*' to be refused with: $want"
    status=1
  elif ! grep -qF -- "$want" <<<"$out"; then
    echo "FAIL: '$*' was refused, but not with: $want"
    echo "$out"
    status=1
  else
    echo "ok: refused $*"
  fi
}

refuse "stellarNetwork" --set config.stellarNetwork=pubnet
refuse "sorobanRpcUrls" --set config.stellarNetwork=mainnet
refuse "mode" --set indexer.mode=many
refuse "/api/image/tag" --set api.image.tag=latest
refuse "repository" --set web.image.repository=
refuse "repository" --set migrations.enabled=true
refuse "database.url" --set database.url= --set database.redisUrl=
refuse "additional properties 'replicas'" --set api.replicas=3
refuse "ingress.tls.secretName" --set ingress.enabled=true --set ingress.tls.enabled=true

exit "$status"
