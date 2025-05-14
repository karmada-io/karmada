{{- define "karmada.webhook.configuration" -}}
{{ $name :=  include "karmada.name" .}}
{{ $namespace := include "karmada.namespace" .}}
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: mutating-config
  labels:
    app: karmada-webhook
    {{- include "karmada.commonLabels" . | nindent 4 }}
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
  - name: resourcebinding.karmada.io
    rules:
      - operations: ["CREATE"]
        apiGroups: ["work.karmada.io"]
        apiVersions: ["*"]
        resources: ["resourcebindings"]
        scope: "Namespaced"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/mutate-resourcebinding
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: clusterresourcebinding.karmada.io
    rules:
      - operations: ["CREATE"]
        apiGroups: ["work.karmada.io"]
        apiVersions: ["*"]
        resources: ["clusterresourcebindings"]
        scope: "Cluster"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/mutate-clusterresourcebinding
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
  - name: multiclusterservice.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["networking.karmada.io"]
        apiVersions: ["*"]
        resources: ["multiclusterservices"]
        scope: "Namespaced"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/mutate-multiclusterservice
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
    app: karmada-webhook
    {{- include "karmada.commonLabels" . | nindent 4 }}
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
  - name: resourcedeletionprotection.karmada.io
    rules:
      - operations: ["DELETE"]
        apiGroups: ["*"]
        apiVersions: ["*"]
        resources: ["*"]
        scope: "*"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/validate-resourcedeletionprotection
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    objectSelector:
      matchExpressions:
        - { key: "resourcetemplate.karmada.io/deletion-protected", operator: "Exists" }
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: [ "v1" ]
    timeoutSeconds: 3
  - name: resourcebinding.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["work.karmada.io"]
        apiVersions: ["*"]
        resources: ["resourcebindings"]
        scope: "Namespaced"
    clientConfig:
      url: https://{{ $name }}-webhook.{{ $namespace }}.svc:443/validate-resourcebinding
      {{- include "karmada.webhook.caBundle" . | nindent 6 }}
    failurePolicy: Fail
    sideEffects: NoneOnDryRun
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
{{- end -}}
