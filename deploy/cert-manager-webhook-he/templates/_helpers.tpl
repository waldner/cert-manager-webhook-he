{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "cert-manager-webhook-he.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cert-manager-webhook-he.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "cert-manager-webhook-he.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "cert-manager-webhook-he.selfSignedIssuer" -}}
{{ printf "%s-selfsign" (include "cert-manager-webhook-he.fullname" .) }}
{{- end -}}

{{- define "cert-manager-webhook-he.rootCAIssuer" -}}
{{ printf "%s-ca" (include "cert-manager-webhook-he.fullname" .) }}
{{- end -}}

{{- define "cert-manager-webhook-he.rootCACertificate" -}}
{{ printf "%s-ca" (include "cert-manager-webhook-he.fullname" .) }}
{{- end -}}

{{- define "cert-manager-webhook-he.servingCertificate" -}}
{{ printf "%s-webhook-tls" (include "cert-manager-webhook-he.fullname" .) }}
{{- end -}}

{{/* Generate a secret reader role + binding */}}
{{- define "cert-manager-webhook-he.secretReaderRole" -}}
{{- $root := index . 0 -}}
{{- $type := index . 1 -}}
{{- $namespace := index . 2 -}}
{{- $roleType := ternary "Role" "ClusterRole" (eq $type "role") -}}
{{- $bindingType := ternary "RoleBinding" "ClusterRoleBinding" (eq $type "role") -}}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: {{ $roleType }}
metadata:
  name: {{ include "cert-manager-webhook-he.fullname" $root }}:secret-reader
{{- if ne $namespace "" }}
  namespace: {{ $namespace }}
{{- end }}
  labels:
    app: {{ include "cert-manager-webhook-he.name" $root }}
    chart: {{ include "cert-manager-webhook-he.chart" $root }}
    release: {{ $root.Release.Name }}
    heritage: {{ $root.Release.Service }}
rules:
  - apiGroups: [""]
    resources:
      - 'secrets'
{{- if $root.Values.rbac.secretNames }}
    resourceNames:
{{- range $name := $root.Values.rbac.secretNames }}
      - {{ $name | quote }}
{{- end }}
{{- end }}
    verbs:
      - 'get'
      - 'watch'
      - 'list'
---
apiVersion: rbac.authorization.k8s.io/v1
kind: {{ $bindingType }}
metadata:
  name: {{ include "cert-manager-webhook-he.fullname" $root }}:secret-reader
{{- if ne $namespace "" }}
  namespace: {{ $namespace }}
{{- end }}
  labels:
    app: {{ include "cert-manager-webhook-he.name" $root }}
    chart: {{ include "cert-manager-webhook-he.chart" $root }}
    release: {{ $root.Release.Name }}
    heritage: {{ $root.Release.Service }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: {{ $roleType }}
  name: {{ include "cert-manager-webhook-he.fullname" $root }}:secret-reader
subjects:
  - apiGroup: ""
    kind: ServiceAccount
    name: {{ include "cert-manager-webhook-he.fullname" $root }}
    namespace: {{ $root.Release.Namespace }}
{{- end -}}
