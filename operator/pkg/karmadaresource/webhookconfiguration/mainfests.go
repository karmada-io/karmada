package webhookconfiguration

const (
	// KarmadaWebhookMutatingWebhookConfiguration is karmada webhook mutatingWebhookConfiguration manifest
	KarmadaWebhookMutatingWebhookConfiguration = `
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
      url: https://{{ .Service }}.{{ .Namespace }}.svc:443/mutate-propagationpolicy
      caBundle: {{ .CaBundle }}
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
      url: https://{{ .Service }}.{{ .Namespace }}.svc:443/mutate-clusterpropagationpolicy
      caBundle: {{ .CaBundle }}
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
      url: https://{{ .Service }}.{{ .Namespace }}.svc:443/mutate-overridepolicy
      caBundle: {{ .CaBundle }}
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
      url: https://{{ .Service }}.{{ .Namespace }}.svc:443/mutate-work
      caBundle: {{ .CaBundle }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
`

	// KarmadaWebhookValidatingWebhookConfiguration is KarmadaWebhook ValidatingWebhookConfiguration manifest
	KarmadaWebhookValidatingWebhookConfiguration = `
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
      url: https://{{ .Service }}.{{ .Namespace }}.svc:443/validate-propagationpolicy
      caBundle: {{ .CaBundle }}
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
      url: https://{{ .Service }}.{{ .Namespace }}.svc:443/validate-clusterpropagationpolicy
      caBundle: {{ .CaBundle }}
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
      url: https://{{ .Service }}.{{ .Namespace }}.svc:443/validate-overridepolicy
      caBundle: {{ .CaBundle }}
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
      url: https://{{ .Service }}.{{ .Namespace }}.svc:443/validate-clusteroverridepolicy
      caBundle: {{ .CaBundle }}
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
      url: https://{{ .Service }}.{{ .Namespace }}.svc:443/validate-resourceinterpreterwebhookconfiguration
      caBundle: {{ .CaBundle }}
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
      url: https://{{ .Service }}.{{ .Namespace }}.svc:443/validate-resourceinterpretercustomization
      caBundle: {{ .CaBundle }}
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
`
)
