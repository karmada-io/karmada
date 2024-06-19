{{/* vim: set filetype=mustache: */}}

{{- define "karmada.name" -}}
{{- default .Release.Name -}}
{{- end -}}

{{- define "karmada.namespace" -}}
{{- default .Release.Namespace -}}
{{- end -}}

{{- define "karmada.commonLabels" -}}
{{- if .Values.global.commonLabels -}}
{{- range $key, $value := .Values.global.commonLabels }}
{{ $key }}: {{ $value | quote }}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "karmada.apiserver.labels" -}}
{{- if .Values.apiServer.labels }}
{{- range $key, $value := .Values.apiServer.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
{{- include "karmada.commonLabels" . -}}
app: {{- include "karmada.name" .}}-apiserver
{{- end }}
{{- end -}}

{{- define "karmada.apiserver.podLabels" -}}
{{- if .Values.apiServer.podLabels }}
{{- range $key, $value := .Values.apiServer.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.etcd.labels" -}}
{{- if .Values.etcd.labels }}
{{- range $key, $value := .Values.etcd.labels }}
{{ $key }}: {{ $value | quote }}
{{- end }}
{{- else}}
{{- include "karmada.commonLabels" . -}}
app: etcd
{{- end }}
{{- end -}}

{{- define "karmada.etcd.podLabels" -}}
{{- if .Values.etcd.podLabels }}
{{- range $key, $value := .Values.etcd.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.aggregatedApiServer.labels" -}}
{{- if .Values.aggregatedApiServer.labels }}
{{- range $key, $value := .Values.aggregatedApiServer.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
app: {{- include "karmada.name" .}}-aggregated-apiserver
{{- end }}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.aggregatedApiServer.podLabels" -}}
{{- if .Values.aggregatedApiServer.podLabels }}
{{- range $key, $value := .Values.aggregatedApiServer.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.metricsAdapter.labels" -}}
{{- if .Values.metricsAdapter.labels }}
{{- range $key, $value := .Values.metricsAdapter.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
app: {{- include "karmada.name" .}}-metrics-adapter
{{- end }}
{{- end -}}

{{- define "karmada.metricsAdapter.podLabels" -}}
{{- if .Values.metricsAdapter.podLabels }}
{{- range $key, $value := .Values.metricsAdapter.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.kube-cm.labels" -}}
{{ $name :=  include "karmada.name" . }}
{{- if .Values.kubeControllerManager.labels -}}
{{- range $key, $value := .Values.kubeControllerManager.labels }}
{{ $key }}: {{ $value }}
{{- end -}}
{{- else -}}
app: {{$name}}-kube-controller-manager
{{- end -}}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.kube-cm.podLabels" -}}
{{- if .Values.kubeControllerManager.podLabels }}
{{- range $key, $value := .Values.kubeControllerManager.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.apiServer.serviceMonitor.labels" -}}
{{- if .Values.serviceMonitor.apiServer.labels }}
{{- range $key, $value := .Values.serviceMonitor.apiServer.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.aggregatedApiServer.serviceMonitor.labels" -}}
{{- if .Values.serviceMonitor.aggregatedApiServer.labels }}
{{- range $key, $value := .Values.serviceMonitor.aggregatedApiServer.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.etcd.serviceMonitor.labels" -}}
{{- if .Values.serviceMonitor.etcd.labels }}
{{- range $key, $value := .Values.serviceMonitor.etcd.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.metricsAdapter.serviceMonitor.labels" -}}
{{- if .Values.serviceMonitor.metricsAdapter.labels }}
{{- range $key, $value := .Values.serviceMonitor.metricsAdapter.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.kubeControllerManager.serviceMonitor.labels" -}}
{{- if .Values.serviceMonitor.kubeControllerManager.labels }}
{{- range $key, $value := .Values.serviceMonitor.kubeControllerManager.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.controllerManager.serviceMonitor.labels" -}}
{{- if .Values.serviceMonitor.controllerManager.labels }}
{{- range $key, $value := .Values.serviceMonitor.controllerManager.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.kubeconfig.volume" -}}
{{- $name := include "karmada.name" . -}}
- name: kubeconfig-secret
  secret:
    secretName: {{ $name }}-kubeconfig
{{- if eq .Values.certs.mode "secrets" }}
- name: kubeconfig-certs
  projected:
    sources:
      - secret:
          name: {{ .Values.certs.secrets.clusterCertSecretName }}
      - configMap:
          name: {{ .Values.certs.secrets.rootCaCrtConfigMap }}
- name: ca-cert-store
  configMap:
    name: {{ .Values.certs.secrets.caBundleConfigMap }}
    defaultMode: 0644
    optional: false
    items:
    - key: ca-certificates.crt
      path: ca-certificates.crt
{{- end }}
{{- end -}}

{{- define "karmada.kubeconfig.volumeMount" -}}
- name: kubeconfig-secret
  subPath: kubeconfig
  mountPath: /etc/kubeconfig
{{- if eq .Values.certs.mode "secrets" }}
- name: kubeconfig-certs
  mountPath: /etc/kubeconfig-certs
  readOnly: true
{{- end }}
{{- end -}}

{{- define "karmada.kubeconfig.caData" -}}
{{- if eq .Values.certs.mode "auto" }}
certificate-authority-data: {{ print "{{ ca_crt }}" }}
{{- end }}
{{- if eq .Values.certs.mode "custom" }}
certificate-authority-data: {{ b64enc .Values.certs.custom.caCrt }}
{{- end }}
{{- if eq .Values.certs.mode "secrets" }}
caBundle: {{ b64enc .Values.certs.secrets.rootCaCrt }}
{{- end }}
{{- end -}}

{{- define "karmada.cm.labels" -}}
{{ $name :=  include "karmada.name" . }}
{{- if .Values.controllerManager.labels -}}
{{- range $key, $value := .Values.controllerManager.labels }}
{{ $key }}: {{ $value }}
{{- end -}}
{{- else -}}
app: {{$name}}-controller-manager
{{- end -}}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.cm.podLabels" -}}
{{- if .Values.controllerManager.podLabels }}
{{- range $key, $value := .Values.controllerManager.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}


{{- define "karmada.scheduler.labels" -}}
{{ $name :=  include "karmada.name" . }}
{{- if .Values.scheduler.labels -}}
{{- range $key, $value := .Values.scheduler.labels }}
{{ $key }}: {{ $value }}
{{- end -}}
{{- else -}}
app: {{$name}}-scheduler
{{- end -}}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.scheduler.podLabels" -}}
{{- if .Values.scheduler.podLabels }}
{{- range $key, $value := .Values.scheduler.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}


{{- define "karmada.descheduler.labels" -}}
{{ $name :=  include "karmada.name" . }}
{{- if .Values.descheduler.labels -}}
{{- range $key, $value := .Values.descheduler.labels }}
{{ $key }}: {{ $value }}
{{- end -}}
{{- else -}}
app: {{$name}}
{{- end -}}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.descheduler.podLabels" -}}
{{- if .Values.descheduler.podLabels }}
{{- range $key, $value := .Values.descheduler.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.descheduler.kubeconfig.volume" -}}
{{ $name :=  include "karmada.name" . }}
{{- if eq .Values.installMode "host" -}}
- name: kubeconfig-secret
  secret:
    secretName: {{ $name }}-kubeconfig
{{- else -}}
- name: kubeconfig-secret
  secret:
    secretName: {{ .Values.descheduler.kubeconfig }}
{{- end -}}
{{- end -}}


{{- define "karmada.webhook.labels" -}}
{{ $name :=  include "karmada.name" .}}
{{- if .Values.webhook.labels }}
{{- range $key, $value := .Values.webhook.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
app: {{$name}}-webhook
{{- end }}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.webhook.podLabels" -}}
{{- if .Values.webhook.podLabels }}
{{- range $key, $value := .Values.webhook.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.agent.labels" -}}
{{ $name :=  include "karmada.name" .}}
{{- if .Values.agent.labels }}
{{- range $key, $value := .Values.agent.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
app: {{$name}}
{{- end }}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.agent.podLabels" -}}
{{- if .Values.agent.podLabels }}
{{- range $key, $value := .Values.agent.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.apiserver.caBundle" -}}
{{- if eq .Values.certs.mode "auto" }}
caBundle: {{ print "{{ ca_crt }}" }}
{{- end }}
{{- if eq .Values.certs.mode "custom" }}
caBundle: {{ b64enc .Values.certs.custom.caCrt }}
{{- end }}
{{- end -}}

{{- define "karmada.webhook.caBundle" -}}
{{- if eq .Values.certs.mode "auto" }}
caBundle: {{ print "{{ ca_crt }}" }}
{{- end }}
{{- if eq .Values.certs.mode "custom" }}
caBundle: {{ b64enc .Values.certs.custom.caCrt }}
{{- end }}
{{- if eq .Values.certs.mode "secrets" }}
caBundle: {{ b64enc .Values.certs.secrets.rootCaCrt }}
{{- end }}
{{- end -}}

{{- define "karmada.schedulerEstimator.podLabels" -}}
{{- if .Values.schedulerEstimator.podLabels }}
{{- range $key, $value := .Values.schedulerEstimator.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.schedulerEstimator.labels" -}}
{{- if .Values.schedulerEstimator.labels }}
{{- range $key, $value := .Values.schedulerEstimator.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.search.labels" -}}
{{- if .Values.search.labels }}
{{- range $key, $value := .Values.search.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
app: {{- include "karmada.name" .}}-search
{{- end }}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.search.podLabels" -}}
{{- if .Values.search.podLabels }}
{{- range $key, $value := .Values.search.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.preInstallJob.labels" -}}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.staticResourceJob.labels" -}}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.postInstallJob.labels" -}}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.postDeleteJob.labels" -}}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.search.kubeconfig.volumeMount" -}}
- name: k8s-certs
  mountPath: /etc/kubernetes/pki
  readOnly: true
- name: etcd-cert
  mountPath: /etc/etcd/pki
  readOnly: true
- name: kubeconfig-secret
  subPath: kubeconfig
  mountPath: /etc/kubeconfig
{{- if eq .Values.certs.mode "secrets" }}
- name: kubeconfig-certs
  mountPath: /etc/kubeconfig-certs
  readOnly: true
- mountPath: /etc/ssl/certs/
  name: ca-cert-store
  readOnly: true
{{- end }}
{{- end -}}

{{- define "karmada.search.kubeconfig.volume" -}}
{{ $name :=  include "karmada.name" . }}
{{- if eq .Values.installMode "host" -}}
- name: k8s-certs
  secret:
    secretName: {{ $name }}-cert
- name: kubeconfig-secret
  secret:
    secretName: {{ $name }}-kubeconfig
{{- else -}}
- name: k8s-certs
  secret:
    secretName: {{ .Values.search.certs }}
- name: kubeconfig-secret
  secret:
    secretName: {{ .Values.search.kubeconfig }}
{{- end -}}
{{- if eq .Values.certs.mode "secrets" }}
- name: kubeconfig-certs
  mountPath: /etc/kubeconfig-certs
  readOnly: true
- name: ca-cert-store
  configMap:
    name: {{ .Values.certs.secrets.caBundleConfigMap }}
    defaultMode: 0644
    optional: false
    items:
    - key: ca-certificates.crt
      path: ca-certificates.crt
{{- end }}
{{- end -}}

{{- define "karmada.search.etcd.cert.volume" -}}
{{ $name :=  include "karmada.name" . }}
- name: etcd-cert
  secret:
  {{- if eq .Values.etcd.mode "internal" }}
    secretName: {{ $name }}-cert
  {{- end }}
  {{- if eq .Values.etcd.mode "external" }}
    secretName: {{ $name }}-external-etcd-cert
  {{- end }}
{{- end -}}

{{/*
Return the proper karmada internal etcd image name
*/}}
{{- define "karmada.internal.etcd.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.etcd.internal.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.internal.etcd.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.etcd.internal.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada agent image name
*/}}
{{- define "karmada.agent.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.agent.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.agent.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.agent.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada apiServer image name
*/}}
{{- define "karmada.apiServer.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.apiServer.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.apiServer.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.apiServer.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada controllerManager image name
*/}}
{{- define "karmada.controllerManager.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.controllerManager.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.controllerManager.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.controllerManager.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada descheduler image name
*/}}
{{- define "karmada.descheduler.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.descheduler.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.descheduler.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.descheduler.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada schedulerEstimator image name
*/}}
{{- define "karmada.schedulerEstimator.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.schedulerEstimator.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.schedulerEstimator.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.schedulerEstimator.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada scheduler image name
*/}}
{{- define "karmada.scheduler.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.scheduler.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.scheduler.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.scheduler.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada webhook image name
*/}}
{{- define "karmada.webhook.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.webhook.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.webhook.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.webhook.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada aggregatedApiServer image name
*/}}
{{- define "karmada.aggregatedApiServer.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.aggregatedApiServer.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.aggregatedApiServer.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.aggregatedApiServer.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada metricsAdapter image name
*/}}
{{- define "karmada.metricsAdapter.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.metricsAdapter.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.metricsAdapter.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.metricsAdapter.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada search image name
*/}}
{{- define "karmada.search.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.search.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.search.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.search.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada kubeControllerManager image name
*/}}
{{- define "karmada.kubeControllerManager.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.kubeControllerManager.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.kubeControllerManager.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.kubeControllerManager.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada cfssl image name
*/}}
{{- define "karmada.cfssl.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.cfssl.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada kubectl image name
*/}}
{{- define "karmada.kubectl.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.kubectl.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.cfssl.image .Values.kubectl.image .Values.etcd.internal.image .Values.agent.image .Values.apiServer.image .Values.controllerManager.image .Values.descheduler.image .Values.schedulerEstimator.image .Values.scheduler.image .Values.webhook.image .Values.aggregatedApiServer.image .Values.metricsAdapter.image .Values.search.image .Values.kubeControllerManager.image) "global" .Values.global) }}
{{- end -}}

{{- define "karmada.controllerManager.featureGates" -}}
     {{- if (not (empty .Values.controllerManager.featureGates)) }}
          {{- $featureGatesFlag := "" -}}
          {{- range $key, $value := .Values.controllerManager.featureGates -}}
               {{- if not (empty (toString $value)) }}
                    {{- $featureGatesFlag = cat $featureGatesFlag $key "=" $value ","  -}}
               {{- end -}}
          {{- end -}}

          {{- if gt (len $featureGatesFlag) 0 }}
               {{- $featureGatesFlag := trimSuffix "," $featureGatesFlag  | nospace -}}
               {{- printf "%s=%s" "--feature-gates" $featureGatesFlag -}}
          {{- end -}}
     {{- end -}}
{{- end -}}

{{- define "karmada.schedulerEstimator.featureGates" -}}
     {{- $featureGatesArg := index . "featureGatesArg" -}}
     {{- if (not (empty $featureGatesArg)) }}
          {{- $featureGatesFlag := "" -}}
          {{- range $key, $value := $featureGatesArg -}}
               {{- if not (empty (toString $value)) }}
                    {{- $featureGatesFlag = cat $featureGatesFlag $key "=" $value ","  -}}
               {{- end -}}
          {{- end -}}

          {{- if gt (len $featureGatesFlag) 0 }}
               {{- $featureGatesFlag := trimSuffix "," $featureGatesFlag  | nospace -}}
               {{- printf "%s=%s" "--feature-gates" $featureGatesFlag -}}
          {{- end -}}
     {{- end -}}
{{- end -}}

{{- define "karmada.controllerManager.extraCommandArgs" -}}
{{- if .Values.controllerManager.extraCommandArgs }}
{{- range $key, $value := .Values.controllerManager.extraCommandArgs }}
- --{{ $key }}={{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.init-sa-secret.volume" -}}
{{- $name := include "karmada.name" . -}}
- name: init-sa-secret
  secret:
    secretName: {{ $name }}-hook-job
{{- end -}}

{{- define "karmada.init-sa-secret.volumeMount" -}}
- name: init-sa-secret
  mountPath: /opt/mount
{{- end -}}

{{- define "karmada.initContainer.build-kubeconfig" -}}
TOKEN=$(cat /opt/mount/token)
kubectl config set-cluster karmada-host --server=https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT} --certificate-authority=/opt/mount/ca.crt
kubectl config set-credentials default --token=$TOKEN
kubectl config set-context karmada-host-context --cluster=karmada-host --user=default --namespace=default
kubectl config use-context karmada-host-context
{{- end -}}

{{- define "karmada.initContainer.waitEtcd" -}}
- name: wait
  image: {{ include "karmada.kubectl.image" . }}
  imagePullPolicy: {{ .Values.kubectl.image.pullPolicy }}
  command:
    - /bin/sh
    - -c
    - |
      bash <<'EOF'
      {{- include "karmada.initContainer.build-kubeconfig" . | nindent 6 }}
      kubectl rollout status statefulset etcd -n {{ include "karmada.namespace" . }}
      EOF
  volumeMounts:
    {{- include "karmada.init-sa-secret.volumeMount" .| nindent 4 }}
{{- end -}}

{{- define "karmada.initContainer.waitStaticResource" -}}
- name: wait
  image: {{ include "karmada.kubectl.image" . }}
  imagePullPolicy: {{ .Values.kubectl.image.pullPolicy }}
  command:
    - /bin/sh
    - -c
    - |
      bash <<'EOF'
      {{- include "karmada.initContainer.build-kubeconfig" . | nindent 6 }}
      kubectl wait --for=condition=complete job {{ include "karmada.name" . }}-static-resource -n {{ include "karmada.namespace" . }}
      EOF
  volumeMounts:
    {{- include "karmada.init-sa-secret.volumeMount" .| nindent 4 }}
{{- end -}}
{{- define "karmada.certs.secrets.etcdVolume" -}}
- name: etcd-cert
  projected:
    sources:
      - secret:
          name: {{ .Values.certs.secrets.clusterCertSecretName }}
          items:
            - key: tls.crt
              path: karmada.crt
            - key: tls.key
              path: karmada.key
            - key: ca.crt
              path: server-ca.crt
{{- end -}}

