# Working with Kyverno

[Kyverno](https://github.com/kyverno/kyverno) , a [Cloud Native Computing Foundation ](https://cncf.io/) project, is a policy engine designed for Kubernetes. It can validate, mutate, and generate configurations using admission controls and background scans. Kyverno policies are Kubernetes resources and do not require learning a new language. Kyverno is designed to work nicely with tools you already use like kubectl, kustomize, and Git.

This document gives an example to demonstrate how to use the `Kyverno` to manage policy.

## Prerequisites
## Setup Karmada

You just need to clone Karmada repo, and run the following script in Karmada directory. 

```
hack/local-up-karmada.sh
```

## Kyverno Installations

In this case, we will use Kyverno v1.6.2. Related deployment files are from [here](https://github.com/kyverno/kyverno/blob/main/config/install.yaml).

### Install Kyverno APIs on Karmada

1. Create resource objects of Kyverno in karmada controller plane, the content is as follows.

   ```console
   kubectl config use-context karmada-apiserver
   ```

   Deploy namespace: https://github.com/kyverno/kyverno/blob/61a1d40e5ea5ff4875a084b6dc3ef1fdcca1ee27/config/install.yaml#L1-L12
   
   Deploy configmap: https://github.com/kyverno/kyverno/blob/61a1d40e5ea5ff4875a084b6dc3ef1fdcca1ee27/config/install.yaml#L12144-L12176
   
   Deploy Kyverno CRDs: https://github.com/kyverno/kyverno/blob/61a1d40e5ea5ff4875a084b6dc3ef1fdcca1ee27/config/install.yaml#L12-L11656

### Install Kyverno components on host cluster

1. Create resource objects of Kyverno in karmada-host context, the content is as follows.

   ```console
   kubectl config use-context karmada-host  
   ```

   Deploy namespace: https://github.com/kyverno/kyverno/blob/61a1d40e5ea5ff4875a084b6dc3ef1fdcca1ee27/config/install.yaml#L1-L12

   Deploy RBAC resources: https://github.com/kyverno/kyverno/blob/61a1d40e5ea5ff4875a084b6dc3ef1fdcca1ee27/config/install.yaml#L11657-L12143

   Deploy Kyverno controllers and service:
   ```yaml
   apiVersion: v1
   kind: Service
   metadata:
     labels:
       app: kyverno
       app.kubernetes.io/component: kyverno
       app.kubernetes.io/instance: kyverno
       app.kubernetes.io/name: kyverno
       app.kubernetes.io/part-of: kyverno
       app.kubernetes.io/version: latest
     name: kyverno-svc
     namespace: kyverno
   spec:
     type: NodePort
     ports:
     - name: https
       port: 443
       targetPort: https
       nodePort: {{nodePort}}
     selector:
       app: kyverno
       app.kubernetes.io/name: kyverno
   ---
   apiVersion: v1
   kind: Service
   metadata:
     labels:
       app: kyverno
       app.kubernetes.io/component: kyverno
       app.kubernetes.io/instance: kyverno
       app.kubernetes.io/name: kyverno
       app.kubernetes.io/part-of: kyverno
       app.kubernetes.io/version: latest
     name: kyverno-svc-metrics
     namespace: kyverno
   spec:
     ports:
     - name: metrics-port
       port: 8000
       targetPort: metrics-port
     selector:
       app: kyverno
       app.kubernetes.io/name: kyverno
   ---
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     labels:
       app: kyverno
       app.kubernetes.io/component: kyverno
       app.kubernetes.io/instance: kyverno
       app.kubernetes.io/name: kyverno
       app.kubernetes.io/part-of: kyverno
       app.kubernetes.io/version: latest
     name: kyverno
     namespace: kyverno
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: kyverno
         app.kubernetes.io/name: kyverno
     strategy:
       rollingUpdate:
         maxSurge: 1
         maxUnavailable: 40%
       type: RollingUpdate
     template:
       metadata:
         labels:
           app: kyverno
           app.kubernetes.io/component: kyverno
           app.kubernetes.io/instance: kyverno
           app.kubernetes.io/name: kyverno
           app.kubernetes.io/part-of: kyverno
           app.kubernetes.io/version: latest
       spec:
         affinity:
           podAntiAffinity:
             preferredDuringSchedulingIgnoredDuringExecution:
             - podAffinityTerm:
                 labelSelector:
                   matchExpressions:
                   - key: app.kubernetes.io/name
                     operator: In
                     values:
                     - kyverno
                 topologyKey: kubernetes.io/hostname
               weight: 1
         containers:
         - args:
           - --filterK8sResources=[Event,*,*][*,kube-system,*][*,kube-public,*][*,kube-node-lease,*][Node,*,*][APIService,*,*][TokenReview,*,*][SubjectAccessReview,*,*][*,kyverno,kyverno*][Binding,*,*][ReplicaSet,*,*][ReportChangeRequest,*,*][ClusterReportChangeRequest,*,*][PolicyReport,*,*][ClusterPolicyReport,*,*]
           - -v=2
           - --autogenInternals=false
           - --kubeconfig=/etc/kubeconfig
           - --serverIP={{nodeIP}}:{{nodePort}}
           env:
           - name: INIT_CONFIG
             value: kyverno
           - name: METRICS_CONFIG
             value: kyverno-metrics
           - name: KYVERNO_NAMESPACE
             valueFrom:
               fieldRef:
                 fieldPath: metadata.namespace
           - name: KYVERNO_SVC
             value: kyverno-svc
           - name: TUF_ROOT
             value: /.sigstore
           image: ghcr.io/kyverno/kyverno:latest
           imagePullPolicy: Always
           livenessProbe:
             failureThreshold: 2
             httpGet:
               path: /health/liveness
               port: 9443
               scheme: HTTPS
             initialDelaySeconds: 15
             periodSeconds: 30
             successThreshold: 1
             timeoutSeconds: 5
           name: kyverno
           ports:
           - containerPort: 9443
             name: https
             protocol: TCP
           - containerPort: 8000
             name: metrics-port
             protocol: TCP
           readinessProbe:
             failureThreshold: 4
             httpGet:
               path: /health/readiness
               port: 9443
               scheme: HTTPS
             initialDelaySeconds: 5
             periodSeconds: 10
             successThreshold: 1
             timeoutSeconds: 5
           resources:
             limits:
               memory: 384Mi
             requests:
               cpu: 100m
               memory: 128Mi
           securityContext:
             allowPrivilegeEscalation: false
             capabilities:
               drop:
               - ALL
             privileged: false
             readOnlyRootFilesystem: true
             runAsNonRoot: true
           volumeMounts:
           - mountPath: /.sigstore
             name: sigstore
           - mountPath: /etc/kubeconfig
             name: kubeconfig
             subPath: kubeconfig
         initContainers:
         - env:
           - name: METRICS_CONFIG
             value: kyverno-metrics
           - name: KYVERNO_NAMESPACE
             valueFrom:
               fieldRef:
                 fieldPath: metadata.namespace
           image: ghcr.io/kyverno/kyvernopre:latest
           imagePullPolicy: Always
           name: kyverno-pre
           resources:
             limits:
               cpu: 100m
               memory: 256Mi
             requests:
               cpu: 10m
               memory: 64Mi
           securityContext:
             allowPrivilegeEscalation: false
             capabilities:
               drop:
               - ALL
             privileged: false
             readOnlyRootFilesystem: true
             runAsNonRoot: true
         securityContext:
           runAsNonRoot: true
         serviceAccountName: kyverno-service-account
         volumes:
         - emptyDir: {}
           name: sigstore
         - name: kubeconfig
           secret:
              defaultMode: 420
              secretName: kubeconfig
   ---
   apiVersion: v1
   stringData:
     kubeconfig: |-
       apiVersion: v1
       clusters:
       - cluster:
           certificate-authority-data: {{ca_crt}}
           server: https://karmada-apiserver.karmada-system.svc.cluster.local:5443
         name: kind-karmada
       contexts:
       - context:
           cluster: kind-karmada
           user: kind-karmada
         name: karmada
       current-context: karmada
       kind: Config
       preferences: {}
       users:
       - name: kind-karmada
         user:
           client-certificate-data: {{client_cer}}
           client-key-data: {{client_key}}
   kind: Secret
   metadata:
     name: kubeconfig
     namespace: kyverno
   ```

   For multi-cluster deployment, We need to add the config of `--serverIP` which is the address of the webhook server. So you need to ensure that the network from node in karmada control plane to those in karmada-host cluster is connected and expose kyverno controller pods to control plane, for example, using `nodePort` above. Then, fill in the secret which represents kubeconfig pointing to karmada-apiserver, such as **ca_crt, client_cer and client_key** above.

## Run demo
### Create require-labels ClusterPolicy

   ClusterPolicy is a CRD which `kyverno` offers to support different kinds of rules. Here is an example ClusterPolicy which means that you must create pod with `app.kubernetes.io/name` label.

   ```console
   kubectl config use-context karmada-apiserver  
   ```
   ```console
   kubectl create -f- << EOF
   apiVersion: kyverno.io/v1
   kind: ClusterPolicy
   metadata:
     name: require-labels
   spec:
     validationFailureAction: enforce
     rules:
     - name: check-for-labels
       match:
         any:
         - resources:
             kinds:
             - Pod
       validate:
         message: "label 'app.kubernetes.io/name' is required"
         pattern:
           metadata:
             labels:
               app.kubernetes.io/name: "?*"
   EOF
   ```

### Create a bad deployment without labels 

   ```console
   kubectl create deployment nginx --image=nginx
   error: failed to create deployment: admission webhook "validate.kyverno.svc-fail" denied the request
   ```

## Reference

- https://github.com/kyverno/kyverno