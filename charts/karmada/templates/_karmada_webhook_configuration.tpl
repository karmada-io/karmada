{{- define "karmada.webhook.configuration" -}}
{{ $name :=  include "karmada.name" .}}
{{ $namespace := include "karmada.namespace" .}}
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: mutating-config
  labels:
    app: mutating-config
webhooks:
  - name: propagationpolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["propagationpolicies"]
        scope: "Namespaced"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/mutate-propagationpolicy
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: clusterpropagationpolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["clusterpropagationpolicies"]
        scope: "Cluster"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/mutate-clusterpropagationpolicy
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: overridepolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["overridepolicies"]
        scope: "Namespaced"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/mutate-overridepolicy
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: work.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["work.karmada.io"]
        apiVersions: ["*"]
        resources: ["works"]
        scope: "Namespaced"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/mutate-work
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: autoscaling.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["autoscaling.karmada.io"]
        apiVersions: ["*"]
        resources: ["federatedhpas"]
        scope: "Namespaced"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/mutate-federatedhpa
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: [ "v1" ]
    timeoutSeconds: 3
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: validating-config
  labels:
    app: validating-config
webhooks:
  - name: propagationpolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["propagationpolicies"]
        scope: "Namespaced"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/validate-propagationpolicy
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: clusterpropagationpolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["clusterpropagationpolicies"]
        scope: "Cluster"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/validate-clusterpropagationpolicy
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: overridepolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["overridepolicies"]
        scope: "Namespaced"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/validate-overridepolicy
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: clusteroverridepolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["clusteroverridepolicies"]
        scope: "Cluster"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/validate-clusteroverridepolicy
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: config.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["config.karmada.io"]
        apiVersions: ["*"]
        resources: ["resourceinterpreterwebhookconfigurations"]
        scope: "Cluster"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/validate-resourceinterpreterwebhookconfiguration
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: resourceinterpretercustomization.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["config.karmada.io"]
        apiVersions: ["*"]
        resources: ["resourceinterpretercustomizations"]
        scope: "Cluster"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/validate-resourceinterpretercustomization
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 10
  - name: federatedresourcequota.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["federatedresourcequotas"]
        scope: "Namespaced"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/validate-federatedresourcequota
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: [ "v1" ]
    timeoutSeconds: 3
  - name: multiclusteringress.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["networking.karmada.io"]
        apiVersions: ["*"]
        resources: ["multiclusteringresses"]
        scope: "Namespaced"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/validate-multiclusteringress
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: multiclusterservice.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["networking.karmada.io"]
        apiVersions: ["*"]
        resources: ["multiclusterservices"]
        scope: "Namespaced"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/validate-multiclusterservice
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: [ "v1" ]
    timeoutSeconds: 3
{{- end -}}
