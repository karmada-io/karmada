# Working with Gatekeeper(OPA)

[Gatekeeper](https://github.com/open-policy-agent/gatekeeper) , is a customizable admission webhook for Kubernetes that enforces policies executed by the Open Policy Agent (OPA), a policy engine for Cloud Native environments hosted by [Cloud Native Computing Foundation](https://cncf.io/).

This document demonstrates how to use the `Gatekeeper` to manage OPA policies. 

## Prerequisites
### Start up Karmada clusters

You just need to clone Karmada repo, and run the following script in the Karmada directory. 

```
hack/local-up-karmada.sh
```

## Gatekeeper Installations

In this case, you will use Gatekeeper v3.7.2. Related deployment files are from [here](https://github.com/open-policy-agent/gatekeeper/blob/release-3.7/deploy/gatekeeper.yaml).

### Install Gatekeeper APIs on Karmada

1. Create resource objects for Gatekeeper in karmada controller plane, the content is as follows. 

   ```console
   kubectl config use-context karmada-apiserver
   ```

   Deploy namespace: https://github.com/open-policy-agent/gatekeeper/blob/0d239574f8e71908325391d49cb8dd8e4ed6f6fa/deploy/gatekeeper.yaml#L1-L9
   
   Deploy Gatekeeper CRDs: https://github.com/open-policy-agent/gatekeeper/blob/0d239574f8e71908325391d49cb8dd8e4ed6f6fa/deploy/gatekeeper.yaml#L27-L1999
   
   Deploy Gatekeeper secrets: https://github.com/open-policy-agent/gatekeeper/blob/0d239574f8e71908325391d49cb8dd8e4ed6f6fa/deploy/gatekeeper.yaml#L2261-L2267
   
   Deploy webhook config:
   ```yaml
   apiVersion: admissionregistration.k8s.io/v1
   kind: MutatingWebhookConfiguration
   metadata:
     labels:
       gatekeeper.sh/system: "yes"
     name: gatekeeper-mutating-webhook-configuration
   webhooks:
     - admissionReviewVersions:
         - v1
         - v1beta1
       clientConfig:
         #Change the clientconfig from service type to url type cause webhook config and service are not in the same cluster.
         url: https://gatekeeper-webhook-service.gatekeeper-system.svc:443/v1/mutate
       failurePolicy: Ignore
       matchPolicy: Exact
       name: mutation.gatekeeper.sh
       namespaceSelector:
         matchExpressions:
           - key: admission.gatekeeper.sh/ignore
             operator: DoesNotExist
       rules:
         - apiGroups:
             - '*'
           apiVersions:
             - '*'
           operations:
             - CREATE
             - UPDATE
           resources:
             - '*'
       sideEffects: None
       timeoutSeconds: 1
   ---
   apiVersion: admissionregistration.k8s.io/v1
   kind: ValidatingWebhookConfiguration
   metadata:
     labels:
       gatekeeper.sh/system: "yes"
     name: gatekeeper-validating-webhook-configuration
   webhooks:
     - admissionReviewVersions:
         - v1
         - v1beta1
       clientConfig:
         #Change the clientconfig from service type to url type cause webhook config and service are not in the same cluster.
         url: https://gatekeeper-webhook-service.gatekeeper-system.svc:443/v1/admit
       failurePolicy: Ignore
       matchPolicy: Exact
       name: validation.gatekeeper.sh
       namespaceSelector:
         matchExpressions:
           - key: admission.gatekeeper.sh/ignore
             operator: DoesNotExist
       rules:
         - apiGroups:
             - '*'
           apiVersions:
             - '*'
           operations:
             - CREATE
             - UPDATE
           resources:
             - '*'
       sideEffects: None
       timeoutSeconds: 3
     - admissionReviewVersions:
         - v1
         - v1beta1
       clientConfig:
         #Change the clientconfig from service type to url type cause webhook config and service are not in the same cluster.
         url: https://gatekeeper-webhook-service.gatekeeper-system.svc:443/v1/admitlabel
       failurePolicy: Fail
       matchPolicy: Exact
       name: check-ignore-label.gatekeeper.sh
       rules:
         - apiGroups:
             - ""
           apiVersions:
             - '*'
           operations:
             - CREATE
             - UPDATE
           resources:
             - namespaces
       sideEffects: None
       timeoutSeconds: 3
   ```
   You need to change the clientconfig from service type to url type for multi-cluster deployment.
   
   Also, you need to deploy a dummy pod in gatekeeper-system namespace in karmada-apiserver context, because when Gatekeeper generates a policy template CRD, a status object is generated to monitor the status of the policy template, and the status object is bound by the controller Pod through the OwnerReference. Therefore, when the CRD and the controller are not in the same cluster, a dummy Pod needs to be used instead of the controller. The Pod enables the status object to be successfully generated. 

   For example: 
   ```yaml
   apiVersion: v1
   kind: Pod
   metadata:
     name: dummy-pod
     namespace: gatekeeper-system
   spec:
     containers:
     - name: dummy-pod
       image: nginx:latest
       imagePullPolicy: Always
   ```

### Install GateKeeper components on host cluster

   ```console
   kubectl config use-context karmada-host  
   ```

   Deploy namespace: https://github.com/open-policy-agent/gatekeeper/blob/0d239574f8e71908325391d49cb8dd8e4ed6f6fa/deploy/gatekeeper.yaml#L1-L9

   Deploy RBAC resources for deployment: https://github.com/open-policy-agent/gatekeeper/blob/0d239574f8e71908325391d49cb8dd8e4ed6f6fa/deploy/gatekeeper.yaml#L1999-L2375
   
   Deploy Gatekeeper controllers and secret as kubeconfig:
   ```yaml 
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     labels:
       control-plane: audit-controller
       gatekeeper.sh/operation: audit
       gatekeeper.sh/system: "yes"
     name: gatekeeper-audit
     namespace: gatekeeper-system
   spec:
     replicas: 1
     selector:
       matchLabels:
         control-plane: audit-controller
         gatekeeper.sh/operation: audit
         gatekeeper.sh/system: "yes"
     template:
       metadata:
         annotations:
           container.seccomp.security.alpha.kubernetes.io/manager: runtime/default
         labels:
           control-plane: audit-controller
           gatekeeper.sh/operation: audit
           gatekeeper.sh/system: "yes"
       spec:
         automountServiceAccountToken: true
         containers:
           - args:
               - --operation=audit
               - --operation=status
               - --operation=mutation-status
               - --logtostderr
               - --disable-opa-builtin={http.send}
               - --kubeconfig=/etc/kubeconfig
             command:
               - /manager
             env:
               - name: POD_NAMESPACE
                 valueFrom:
                   fieldRef:
                     apiVersion: v1
                     fieldPath: metadata.namespace
               - name: POD_NAME
                 value: {{POD_NAME}}
             image: openpolicyagent/gatekeeper:v3.7.2
             imagePullPolicy: Always
             livenessProbe:
               httpGet:
                 path: /healthz
                 port: 9090
             name: manager
             ports:
               - containerPort: 8888
                 name: metrics
                 protocol: TCP
               - containerPort: 9090
                 name: healthz
                 protocol: TCP
             readinessProbe:
               httpGet:
                 path: /readyz
                 port: 9090
             resources:
               limits:
                 cpu: 1000m
                 memory: 512Mi
               requests:
                 cpu: 100m
                 memory: 256Mi
             securityContext:
               allowPrivilegeEscalation: false
               capabilities:
                 drop:
                   - all
               readOnlyRootFilesystem: true
               runAsGroup: 999
               runAsNonRoot: true
               runAsUser: 1000
             volumeMounts:
               - mountPath: /tmp/audit
                 name: tmp-volume
               - mountPath: /etc/kubeconfig
                 name: kubeconfig
                 subPath: kubeconfig
         nodeSelector:
           kubernetes.io/os: linux
         priorityClassName: system-cluster-critical
         serviceAccountName: gatekeeper-admin
         terminationGracePeriodSeconds: 60
         volumes:
           - emptyDir: {}
             name: tmp-volume
           - name: kubeconfig
             secret:
               defaultMode: 420
               secretName: kubeconfig
   ---
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     labels:
       control-plane: controller-manager
       gatekeeper.sh/operation: webhook
       gatekeeper.sh/system: "yes"
     name: gatekeeper-controller-manager
     namespace: gatekeeper-system
   spec:
     replicas: 3
     selector:
       matchLabels:
         control-plane: controller-manager
         gatekeeper.sh/operation: webhook
         gatekeeper.sh/system: "yes"
     template:
       metadata:
         annotations:
           container.seccomp.security.alpha.kubernetes.io/manager: runtime/default
         labels:
           control-plane: controller-manager
           gatekeeper.sh/operation: webhook
           gatekeeper.sh/system: "yes"
       spec:
         affinity:
           podAntiAffinity:
             preferredDuringSchedulingIgnoredDuringExecution:
               - podAffinityTerm:
                   labelSelector:
                     matchExpressions:
                       - key: gatekeeper.sh/operation
                         operator: In
                         values:
                           - webhook
                   topologyKey: kubernetes.io/hostname
                 weight: 100
         automountServiceAccountToken: true
         containers:
           - args:
               - --port=8443
               - --logtostderr
               - --exempt-namespace=gatekeeper-system
               - --operation=webhook
               - --operation=mutation-webhook
               - --disable-opa-builtin={http.send}
               - --kubeconfig=/etc/kubeconfig
             command:
               - /manager
             env:
               - name: POD_NAMESPACE
                 valueFrom:
                   fieldRef:
                     apiVersion: v1
                     fieldPath: metadata.namespace
               - name: POD_NAME
                 value: {{POD_NAME}}
             image: openpolicyagent/gatekeeper:v3.7.2
             imagePullPolicy: Always
             livenessProbe:
               httpGet:
                 path: /healthz
                 port: 9090
             name: manager
             ports:
               - containerPort: 8443
                 name: webhook-server
                 protocol: TCP
               - containerPort: 8888
                 name: metrics
                 protocol: TCP
               - containerPort: 9090
                 name: healthz
                 protocol: TCP
             readinessProbe:
               httpGet:
                 path: /readyz
                 port: 9090
             resources:
               limits:
                 cpu: 1000m
                 memory: 512Mi
               requests:
                 cpu: 100m
                 memory: 256Mi
             securityContext:
               allowPrivilegeEscalation: false
               capabilities:
                 drop:
                   - all
               readOnlyRootFilesystem: true
               runAsGroup: 999
               runAsNonRoot: true
               runAsUser: 1000
             volumeMounts:
               - mountPath: /certs
                 name: cert
                 readOnly: true
               - mountPath: /etc/kubeconfig
                 name: kubeconfig
                 subPath: kubeconfig
         nodeSelector:
           kubernetes.io/os: linux
         priorityClassName: system-cluster-critical
         serviceAccountName: gatekeeper-admin
         terminationGracePeriodSeconds: 60
         volumes:
           - name: cert
             secret:
               defaultMode: 420
               secretName: gatekeeper-webhook-server-cert
           - name: kubeconfig
             secret:
               defaultMode: 420
               secretName: kubeconfig
   ---
   apiVersion: policy/v1beta1
   kind: PodDisruptionBudget
   metadata:
     labels:
       gatekeeper.sh/system: "yes"
     name: gatekeeper-controller-manager
     namespace: gatekeeper-system
   spec:
     minAvailable: 1
     selector:
       matchLabels:
         control-plane: controller-manager
         gatekeeper.sh/operation: webhook
         gatekeeper.sh/system: "yes"
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
     namespace: gatekeeper-system
   ``` 
   You need to fill in the dummy pod created in step 1 to {{ POD_NAME }} and fill in the secret which represents kubeconfig pointing to karmada-apiserver.
   
   Deploy ResourceQuota: https://github.com/open-policy-agent/gatekeeper/blob/0d239574f8e71908325391d49cb8dd8e4ed6f6fa/deploy/gatekeeper.yaml#L10-L26

### Extra steps

   After all, we need to copy the secret `gatekeeper-webhook-server-cert` in karmada-apiserver context to that in karmada-host context to keep secrets stored in `etcd` and volumes mounted in controller the same.

## Run demo
### Create k8srequiredlabels template

   ```yaml
   apiVersion: templates.gatekeeper.sh/v1
   kind: ConstraintTemplate
   metadata:
     name: k8srequiredlabels
   spec:
     crd:
       spec:
         names:
           kind: K8sRequiredLabels
         validation:
           openAPIV3Schema:  
             type: object
             description: Describe K8sRequiredLabels crd parameters
             properties:
               labels:
                 type: array
                 items:
                   type: string
                   description: A label string
     targets:
       - target: admission.k8s.gatekeeper.sh
         rego: |
           package k8srequiredlabels
   
           violation[{"msg": msg, "details": {"missing_labels": missing}}] {
             provided := {label | input.review.object.metadata.labels[label]}
             required := {label | label := input.parameters.labels[_]}
             missing := required - provided
             count(missing) > 0
             msg := sprintf("you must provide labels: %v", [missing])
           }
   ```

### Create k8srequiredlabels constraint

   ```yaml
   apiVersion: constraints.gatekeeper.sh/v1beta1
   kind: K8sRequiredLabels
   metadata:
     name: ns-must-have-gk
   spec:
     match:
       kinds:
         - apiGroups: [""]
           kinds: ["Namespace"]  
     parameters:
       labels: ["gatekeepers"]  
   ```

### Create a bad namespace 

   ```console
   kubectl create ns test
   Error from server ([ns-must-have-gk] you must provide labels: {"gatekeepers"}): admission webhook "validation.gatekeeper.sh" denied the request: [ns-must-have-gk] you must provide labels: {"gatekeepers"}
   ```

## Reference

- https://github.com/open-policy-agent/gatekeeper