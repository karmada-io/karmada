package estimator

const (
	karmadaEstimatorDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karmada-scheduler-estimator-{{ .MemberClusterName}}
  namespace: {{ .Namespace }}
  labels:
    cluster: {{ .MemberClusterName}}
spec:
  selector:
    matchLabels:
      app: karmada-scheduler-estimator-{{ .MemberClusterName}}
  replicas: {{ .Replicas }}
  template:
    metadata:
      labels:
        app: karmada-scheduler-estimator-{{ .MemberClusterName}}
        cluster: {{ .MemberClusterName}}
    spec:
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      automountServiceAccountToken: false
      containers:
        - name: karmada-scheduler-estimator
          image: {{ .Image }}
          imagePullPolicy: IfNotPresent
          command:
            - /bin/karmada-scheduler-estimator
            - --kubeconfig=/etc/{{ .MemberClusterName}}-kubeconfig
            - --cluster-name={{ .MemberClusterName}}
          livenessProbe:
            httpGet:
              path: /healthz
              port: 10351
              scheme: HTTP
            failureThreshold: 3
            initialDelaySeconds: 15
            periodSeconds: 15
            timeoutSeconds: 5
          volumeMounts:
            - name: member-kubeconfig
              subPath: {{ .MemberClusterName}}-kubeconfig
              mountPath: /etc/{{ .MemberClusterName}}-kubeconfig
      volumes:
        - name: member-kubeconfig
          secret:
            secretName: {{ .MemberClusterName}}-kubeconfig

`

	karmadaEstimatorService = `
apiVersion: v1
kind: Service
metadata:
  name: karmada-scheduler-estimator-{{ .MemberClusterName}}
  namespace: {{ .Namespace }}
  labels:
    cluster: {{ .MemberClusterName}}
spec:
  selector:
    app: karmada-scheduler-estimator-{{ .MemberClusterName}}
  ports:
    - protocol: TCP
      port: 10352
      targetPort: 10352
`
)

// DeploymentReplace is a struct to help to concrete
// the karamda-estimator deployment bytes with the deployment template
type DeploymentReplace struct {
	Namespace         string
	Replicas          *int32
	Image             string
	MemberClusterName string
}

// ServiceReplace is a struct to help to concrete
// the karamda-estimator Service bytes with the Service template
type ServiceReplace struct {
	Namespace         string
	MemberClusterName string
}
