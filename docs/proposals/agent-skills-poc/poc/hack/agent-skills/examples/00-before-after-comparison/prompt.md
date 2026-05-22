# The prompt

The same natural-language request was given to a generic LLM (no Karmada skill loaded)
and to the same LLM with `karmada-create-policy` loaded.

## User prompt

> Make a Karmada policy for this Deployment. Place it on `member1` and `member2`.
> Use the image registry `mirror.id.internal` on `member2` only.

## Attached workload (`workload.yaml`)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: frontend-config
  namespace: default
data:
  theme: dark-mode
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: global-frontend
  namespace: default
  labels:
    app: global-frontend
spec:
  replicas: 6
  selector:
    matchLabels:
      app: global-frontend
  template:
    metadata:
      labels:
        app: global-frontend
    spec:
      containers:
        - name: web
          image: docker.io/library/nginx:1.27
          envFrom:
            - configMapRef:
                name: frontend-config
```
