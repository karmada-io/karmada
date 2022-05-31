# 离线部署multi-cluster-ingress
在已有的k8s集群上通过helm离线部署muti-cluster-ingress
# 前期准备
- 安装helm
- 安装karmada
- master集群和member集群之间的容器网络互通，可以使用Submariner或其他相关的开源项目来连接集群之间的网络。

# 构建镜像
目前karmada没有提供muti-cluster-ingress的镜像，需要在一个能连外网的环境上编译构建。
### 下载代码

```
// for HTTPS
git clone https://github.com/karmada-io/multi-cluster-ingress-nginx.git
// for SSH
git clone git@github.com:karmada-io/multi-cluster-ingress-nginx.git
```
### 编译和构建镜像

```
make build image
```
如果不方便构建镜像的小伙伴，可以从这边下载

```
docker pull luomonkeyking/ingress-nginx-controller:v1.1.1

//kube-webhook-certgen是官方镜像不需要自己编译
docker pull luomonkeyking/kube-webhook-certgen:v1.1.1

//hello-app的镜像是官方例子，可以根据需要下载
docker pull luomonkeyking/hello-app:1.0
```

# 部署
在master集群上通过helm部署，需要用到multi-cluster-ingress源码中的charts部分，所以需要将源码下载到环境中
### 下载代码

```
// for HTTPS
git clone https://github.com/karmada-io/multi-cluster-ingress-nginx.git
// for SSH
git clone git@github.com:karmada-io/multi-cluster-ingress-nginx.git
```
### 修改charts文件
这里我把charts目录下values.yaml中的镜像digest给注释了，不然识别不了(这里不是很熟悉，欢迎了解的小伙伴补充),同时需要将上面构建的镜像通过docker tag的方式修改成和value.yaml中的一致。

```
     10 controller:
     11   name: controller
     12   image:
     13     registry: k8s.gcr.io
     14     image: ingress-nginx/controller
     15     ## for backwards compatibility consider setting the full image url via the repository value below
     16     ## use *either* current default registry/image or repository format or installing chart by providing the values.yaml will fail
     17     ## repository:
     18     tag: "v1.1.1"
            //这里注释掉了
     19     #digest: sha256:0bc88eb15f9e7f84e8e56c14fa5735aaa488b840983f87bd79b1054190e660de
     20     pullPolicy: IfNotPresent
     
     ...
     
    604     patch:
    605       enabled: true
    606       image:
    607         registry: k8s.gcr.io
    608         image: ingress-nginx/kube-webhook-certgen
    609         ## for backwards compatibility consider setting the full image url via the repository value below
    610         ## use *either* current default registry/image or repository format or installing chart by providing the values.yaml will fai        l
    611         ## repository:
    612         tag: v1.1.1
                //这里注释掉了
    613         #digest: sha256:64d8c73dca984af206adf9d6d7e46aa550362b1d7a01f3a0a91b20cc67868660
    614         pullPolicy: IfNotPresent
```

### 通过helm离线部署
在multi-cluster-ingress-nginx/charts目录下进行安装

```
helm install ingress-nginx -n ingress-nginx ./ingress-nginx
```

# 测试应用
测试应用可以参考官方文档[multi-cluster-ingress](https://github.com/karmada-io/karmada/blob/master/docs/multi-cluster-ingress.md)

### FAQ
- 在测试应用的最后一步通过port-forward端口转发时没有添加--address参数时，默认走的是127.0.0.1，所以在hosts中添加域名映射时，需要加到127.0.0.1后面
```
127.0.0.1   localhost localhost.localdomain localhost4 demo.localdev.me
```
如果想要将域名映射添加到主机ip后面的话，可以在命令中添加--address参数

```
kubectl port-forward --address 0.0.0.0 --namespace=ingress-nginx service/ingress-nginx-controller 8080:80
```

