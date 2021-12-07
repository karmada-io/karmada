# Troubleshooting

## I can't access some resources when install Karmada

- Pulling images from Google Container Registry(k8s.gcr.io)

You may run the following command to change the image registry in the mainland China
```shell
sed -i'' -e "s#k8s.gcr.io#registry.aliyuncs.com/google_containers#g" artifacts/deploy/karmada-etcd.yaml
sed -i'' -e "s#k8s.gcr.io#registry.aliyuncs.com/google_containers#g" artifacts/deploy/karmada-apiserver.yaml
sed -i'' -e "s#k8s.gcr.io#registry.aliyuncs.com/google_containers#g" artifacts/deploy/kube-controller-manager.yaml
```  
- Download golang package in the mainland China, run the following command before installing
```shell
export GOPROXY=https://goproxy.cn