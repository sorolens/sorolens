{{/* Chart name, truncated to the 63 characters a label allows. */}}
{{- define "sorolens.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* Fully qualified prefix for every resource name. */}}
{{- define "sorolens.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/* Name of one component's resources: <fullname>-<component>. */}}
{{- define "sorolens.componentName" -}}
{{- printf "%s-%s" (include "sorolens.fullname" .root) .component | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "sorolens.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* Labels on every resource. */}}
{{- define "sorolens.labels" -}}
helm.sh/chart: {{ include "sorolens.chart" . }}
app.kubernetes.io/name: {{ include "sorolens.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: sorolens
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/* Selector labels for one component. Call with (dict "root" $ "component" "api"). */}}
{{- define "sorolens.selectorLabels" -}}
app.kubernetes.io/name: {{ include "sorolens.name" .root }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{/* All labels for one component's resources. */}}
{{- define "sorolens.componentLabels" -}}
{{ include "sorolens.labels" .root }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{- define "sorolens.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "sorolens.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/* image reference: repository:tag, the tag defaulting to appVersion. */}}
{{- define "sorolens.image" -}}
{{- printf "%s:%s" .image.repository (default .root.Chart.AppVersion .image.tag) }}
{{- end }}

{{/* Secret holding the connection strings. */}}
{{- define "sorolens.secretName" -}}
{{- if .Values.database.existingSecret }}
{{- .Values.database.existingSecret }}
{{- else }}
{{- include "sorolens.componentName" (dict "root" . "component" "connections") }}
{{- end }}
{{- end }}

{{/* Public SDF RPC endpoints; mainnet deliberately has none. */}}
{{- define "sorolens.rpcUrls" -}}
{{- $defaults := dict "testnet" "https://soroban-testnet.stellar.org:443" "futurenet" "https://soroban-futurenet.stellar.org:443" }}
{{- toYaml (merge (deepCopy .Values.config.sorobanRpcUrls) $defaults) }}
{{- end }}

{{/* Networks the indexer polls: the default network plus any configured. */}}
{{- define "sorolens.indexedNetworks" -}}
{{- $networks := list .Values.config.stellarNetwork }}
{{- range $network, $_ := .Values.config.sorobanRpcUrls }}
{{- $networks = append $networks $network }}
{{- end }}
{{- toYaml (uniq $networks) }}
{{- end }}

{{/* SOROBAN_RPC_URL_<NETWORK> for every indexed network. */}}
{{- define "sorolens.perNetworkRpcEnv" -}}
{{- $urls := include "sorolens.rpcUrls" . | fromYaml }}
{{- range $network := (include "sorolens.indexedNetworks" . | fromYamlArray) }}
{{- if not (hasKey $urls $network) }}
{{- fail (printf "config.sorobanRpcUrls must include an endpoint for %q: there is no public default for it" $network) }}
{{- end }}
- name: SOROBAN_RPC_URL_{{ upper $network }}
  value: {{ get $urls $network | quote }}
{{- end }}
{{- end }}

{{/* Connection string env vars read from the Secret. */}}
{{- define "sorolens.connectionEnv" -}}
- name: DATABASE_URL
  valueFrom:
    secretKeyRef:
      name: {{ include "sorolens.secretName" . }}
      key: {{ .Values.database.secretKeys.databaseUrl }}
- name: REDIS_URL
  valueFrom:
    secretKeyRef:
      name: {{ include "sorolens.secretName" . }}
      key: {{ .Values.database.secretKeys.redisUrl }}
{{- end }}

{{/* Annotation that restarts pods when the chart-managed Secret changes. */}}
{{- define "sorolens.checksumAnnotations" -}}
{{- if not .Values.database.existingSecret }}
checksum/connections: {{ include (print .Template.BasePath "/secret.yaml") . | sha256sum }}
{{- end }}
{{- end }}
