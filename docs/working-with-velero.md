# Integrate Velero to back up and restore Karmada resources

[Velero](https://github.com/vmware-tanzu/velero) gives you tools to back up and restore 
your Kubernetes cluster resources and persistent volumes. You can run Velero with a public
cloud platform or on-premises.

Velero lets you:

- Take backups of your cluster and restore in case of loss.
- Migrate cluster resources to other clusters.
- Replicate your production cluster to development and testing clusters.

This document gives an example to demonstrate how to use the `Velero` to back up and restore Kubernetes cluster resources
and persistent volumes. Following example backups resources in cluster `member1`, and then restores those to cluster `member2`.

## Start up Karamda clusters
You just need to clone Karamda repo, and run the following script in Karamda directory.
```shell
hack/local-up-karmada.sh
```

And then run the below command to switch to the member cluster `member1`.
```shell
export KUBECONFIG=/root/.kube/members.config
kubectl config use-context member1
```

## Install MinIO
Velero uses Object Storage Services from different cloud providers to support backup and snapshot operations.
For simplicity, here takes, one object storage that runs locally on k8s clusters, for an example.

Download the binary from the official site:
```shell
wget https://dl.min.io/server/minio/release/linux-amd64/minio
chmod +x minio
```

Run the below command to set `MinIO` username and password:
```shell
export MINIO_ROOT_USER=minio
export MINIO_ROOT_PASSWORD=minio123
```

Run this command to start `MinIO`:
```shell
./minio server /data --console-address="0.0.0.0:20001" --address="0.0.0.0:9000"
```

Replace `/data` with the path to the drive or directory in which you want `MinIO` to store data. And now we can visit 
`http://{SERVER_EXTERNAL_IP}/20001` in the browser to visit `MinIO` console UI. And `Velero` can use
`http://{SERVER_EXTERNAL_IP}/9000` to connect `MinIO`. The two configuration will make our follow-up work easier and more convenient.

Please visit `MinIO` console to create region `minio` and bucket `velero`, these will be used by `Velero`.

For more details about how to install `MinIO`, please run `minio server --help` for help, or you can visit
[MinIO Github Repo](https://github.com/minio/minio).

## Install Velero
Velero consists of two components:
- ### A command-line client that runs locally.
  1. Download the [release](https://github.com/vmware-tanzu/velero/releases) tarball for your client platform
  ```shell
  wget https://github.com/vmware-tanzu/velero/releases/download/v1.7.0/velero-v1.7.0-linux-amd64.tar.gz
  ```
  
  2. Extract the tarball:
  ```shell
  tar -zxvf velero-v1.7.0-linux-amd64.tar.gz
  ```
  
  3. Move the extracted velero binary to somewhere in your $PATH (/usr/local/bin for most users).
  ```shell
  cp velero-v1.7.0-linux-amd64/velero /usr/local/bin/
  ```

- ### A server that runs on your cluster
   We will use `velero install` to set up server components.
  
   For more details about how to use `MinIO` and `Velero` to backup resources, please ref: https://velero.io/docs/v1.7/contributions/minio/
  
   1. Create a Velero-specific credentials file (credentials-velero) in your local directory:
   ```shell
   [default]
   aws_access_key_id = minio
   aws_secret_access_key = minio123
   ```
   The two values should keep the same with `MinIO` username and password that we set when we install `MinIO`
  
   2. Start the server.
   
   We need to install `Velero` in both `member1` and `member2`, so we should run the below command in shell for both two clusters,
   this will start Velero server. Please run `kubectl config use-context member1` and `kubectl config use-context member2`
   to switch to the different member clusters: `member1` or `member2`.
   ```shell
   velero install \
   --provider aws \
   --plugins velero/velero-plugin-for-aws:v1.2.1 \
   --bucket velero \
   --secret-file ./credentials-velero \
   --use-volume-snapshots=false \
   --backup-location-config region=minio,s3ForcePathStyle="true",s3Url=http://{SERVER_EXTERNAL_IP}:9000
   ```
   Replace `{SERVER_EXTERNAL_IP}` with your own server external IP.

   3. Deploy the nginx application to cluster `member1`: 
  
   Run the below command in the Karmada directory.
   ```shell
   kubectl apply -f samples/nginx/deployment.yaml
   ```
  
   And then you will find nginx is deployed successfully.
   ```shell
   # kubectl get deployment.apps
   NAME    READY   UP-TO-DATE   AVAILABLE   AGE
   nginx   2/2     2            2           17s
   ```

### Back up and restore kubernetes resources independent
Create a backup in `member1`:
```shell
velero backup create nginx-backup --selector app=nginx
```

Restore the backup in `member2`

Run this command to switch to `member2`
```shell
export KUBECONFIG=/root/.kube/members.config
kubectl config use-context member2
```

In `member2`, we can also get the backup that we created in `member1`:
```shell
# velero backup get
NAME           STATUS      ERRORS   WARNINGS   CREATED                         EXPIRES   STORAGE LOCATION   SELECTOR
nginx-backup   Completed   0        0          2021-12-10 15:16:46 +0800 CST   29d       default            app=nginx
```

Restore `member1` resources to `member2`:
```shell
# velero restore create --from-backup nginx-backup
Restore request "nginx-backup-20211210151807" submitted successfully.
```

Watch restore result, you'll find that the status is Completed.
```shell
# velero restore get
NAME                          BACKUP         STATUS      STARTED                         COMPLETED                       ERRORS   WARNINGS   CREATED                         SELECTOR
nginx-backup-20211210151807   nginx-backup   Completed   2021-12-10 15:18:07 +0800 CST   2021-12-10 15:18:07 +0800 CST   0        0          2021-12-10 15:18:07 +0800 CST   <none>
```

And then you can find deployment nginx will be restored successfully. 
```shell
# kubectl get deployment.apps/nginx
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   2/2     2            2           21s
```

### Backup and restore of kubernetes resources through Velero combined with karmada

In Karmada control plane, we need to install velero crds but do not need controllers to reconcile them. They are treated as resource templates, not specific resource instances.Based on work API here, they will be encapsulated as a work object deliverd to member clusters and reconciled by velero controllers in member clusters finally.

Create velero crds in Karmada control plane: 
remote velero crd directory: `https://github.com/vmware-tanzu/helm-charts/tree/main/charts/velero/crds/`

Create a backup in `karmada-apiserver` and Distributed to `member1` cluster through PropagationPolicy

```shell
# create backup policy
cat <<EOF | kubectl apply -f -
apiVersion: velero.io/v1
kind: Backup
metadata:
  name:  member1-default-backup
  namespace: velero
spec:
  defaultVolumesToRestic: false
  includedNamespaces:
  - default
  storageLocation: default
EOF

# create PropagationPolicy
cat <<EOF | kubectl apply -f -
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: member1-backup
  namespace: velero
spec:
  resourceSelectors:
    - apiVersion: velero.io/v1
      kind: Backup
  placement:
    clusterAffinity:
      clusterNames:
        - member1
EOF
```


Create a restore in `karmada-apiserver` and Distributed to `member2` cluster through PropagationPolicy

```shell
#create restore policy
cat <<EOF | kubectl apply -f -
apiVersion: velero.io/v1
kind: Restore
metadata:
  name: member1-default-restore
  namespace: velero
spec:
  backupName: member1-default-backup
  excludedResources:
  - nodes
  - events
  - events.events.k8s.io
  - backups.velero.io
  - restores.velero.io
  - resticrepositories.velero.io
  hooks: {}
  includedNamespaces:
  - 'default'
EOF

#create PropagationPolicy
cat <<EOF | kubectl apply -f -
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: member2-default-restore-policy
  namespace: velero
spec:
  resourceSelectors:
    - apiVersion: velero.io/v1
      kind: Restore
  placement:
    clusterAffinity:
      clusterNames:
        - member2
EOF

```

And then you can find deployment nginx will be restored on member2 successfully. 
```shell
# kubectl get deployment.apps/nginx
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   2/2     2            2           10s
```


## Reference
The above introductions about `Velero` and `MinIO` are only a summary from the official website and repos, for more details please refer to:
- Velero: https://velero.io/
- MinIO: https://min.io/
