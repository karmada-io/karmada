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
{{- if .Values.kubeControllerManager.labels }}
{{- range $key, $value := .Values.kubeControllerManager.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
app: {{- include "karmada.name" .}}-kube-controller-manager
{{- end }}
{{- include "karmada.commonLabels" . -}}
{{- end -}}

{{- define "karmada.kube-cm.podLabels" -}}
{{- if .Values.kubeControllerManager.podLabels }}
{{- range $key, $value := .Values.kubeControllerManager.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.kubeconfig.volume" -}}
{{- $name := include "karmada.name" . -}}
- name: kubeconfig-secret
  secret:
    secretName: {{ $name }}-kubeconfig
{{- end -}}

{{- define "karmada.kubeconfig.volumeMount" -}}
- name: kubeconfig-secret
  subPath: kubeconfig
  mountPath: /etc/kubeconfig
{{- end -}}

{{- define "karmada.kubeconfig.caData" -}}
{{- if eq .Values.certs.mode "auto" }}
certificate-authority-data: {{ print "{{ ca_crt }}" }}
{{- end }}
{{- if eq .Values.certs.mode "custom" }}
certificate-authority-data: {{ b64enc .Values.certs.custom.caCrt }}
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

{{- define "karmada.preUpdateJob.labels" -}}
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
{{- end -}}

{{- define "karmada.search.etcd.cert.volume" -}}
{{ $name :=  include "karmada.name" . }}
- name: etcd-certs
  secret:
  {{- if eq .Values.etcd.mode "internal" }}
    secretName: {{ $name }}-cert
  {{- end }}
  {{- if eq .Values.etcd.mode "external" }}
    secretName: {{ $name }}-external-etcd-cert
  {{- end }}
{{- end -}}

{{- define "karmada.scheduler.cert.volume" -}}
{{ $name :=  include "karmada.name" . }}
- name: karmada-certs
  secret:
    secretName: {{ $name }}-cert
{{- end -}}

{{/*
Common env for POD_IP
*/}}
{{- define "karmada.env.podIP" -}}
- name: POD_IP
  valueFrom:
    fieldRef:
      fieldPath: status.podIP
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

{{- /* 
Generate the --feature-gates command line argument for karmada-webhook.
Iterates over .Values.webhook.featureGates and constructs a comma-separated key=value list.
If any feature gates are set, outputs: --feature-gates=Foo=true,Bar=false
If none are set, outputs nothing.
*/ -}}
{{- define "karmada.webhook.featureGates" -}}
     {{- if (not (empty .Values.webhook.featureGates)) }}
          {{- $featureGatesFlag := "" -}}
          {{- range $key, $value := .Values.webhook.featureGates -}}
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

{{- define "karmada.initContainer.waitEtcd" -}}
- name: wait
  image: {{ include "karmada.cfssl.image" . }}
  imagePullPolicy: {{ .Values.kubectl.image.pullPolicy }}
  command:
    - /bin/sh
    - -c
    - |
      bash <<'EOF'
      set -ex
      while true; do
        ETCD_ENDPOINT=${ETCD_CLIENT_SERVICE_HOST}":"${ETCD_CLIENT_SERVICE_PORT}

        # check etcd connectivity by executing curl.
        # if etcd is ready, the response of curl would be `curl: (52) Empty reply from server`, with return code 52.
        # if not, the response of curl would be like `curl: (7) Failed to connect to .....`, with other return code.
        if curl --connect-timeout 2 ${ETCD_ENDPOINT} || [ $? -eq 52 ]; then
          break
        fi

        echo "failed to connect to "${ETCD_ENDPOINT}
        sleep 2
      done
      echo "successfully connect to "${ETCD_ENDPOINT}
      EOF
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
      set -ex

      # here are three cases:
      # case first installation: no `cm/karmada-version` at first, so when you get it, it means `karmada-static-resource-job` finished.
      # case restart: already has `cm/karmada-version`, which means `karmada-static-resource-job` already finished.
      # case upgrading: already has `cm/karmada-version`, but it may be old version, we should wait until `.data.karmadaVersion` equal to current `.Values.karmadaImageVersion`.
      while [[ $(kubectl --kubeconfig /etc/kubeconfig get configmap karmada-version -n {{ .Values.systemNamespace }} -o jsonpath='{.data.karmadaVersion}') != {{ .Values.karmadaImageVersion }} ]]; do
        echo "wait for karmada-static-resource-job finished"; sleep 2
      done

      echo "karmada-static-resource-job successfully completed since expected configmap value was found"
      EOF
  volumeMounts:
    {{- include "karmada.kubeconfig.volumeMount" .| nindent 4 }}
{{- end -}}
