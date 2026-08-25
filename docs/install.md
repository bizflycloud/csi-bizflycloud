# Install Bizfly Cloud CSI Drivers

Bizfly Cloud provides two CSI drivers for Kubernetes:
1. **[File Storage CSI Driver (`fs.csi.bizflycloud.vn`)](#file-storage-csi-driver-readwritemany)** — Shared NFS-backed storage supporting **ReadWriteMany (RWX)**.
2. **[Block Storage CSI Driver (`volume.csi.bizflycloud.vn`)](#block-storage-csi-driver-readwriteonce)** — Dedicated block volume storage supporting **ReadWriteOnce (RWO)**.

---

## File Storage CSI Driver (ReadWriteMany)

For full step-by-step instructions with examples, see **[File Storage Installation Guide](install-filestorage.md)**.

### Quick Summary:

1. **Install prerequisite `csi-driver-nfs` via Helm**:
   ```bash
   helm repo add csi-driver-nfs https://raw.githubusercontent.com/kubernetes-csi/csi-driver-nfs/master/charts
   helm repo update
   helm install csi-driver-nfs csi-driver-nfs/csi-driver-nfs \
       --namespace kube-system \
       --set feature.enableFSGroupPolicy=true
   ```

2. **Create Secret `bizflycloud` in `kube-system`**:
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: bizflycloud
     namespace: kube-system
   stringData:
     application_credential_id: "<your_application_credential_id>"
     application_credential_secret: "<your_application_credential_secret>"
     tenant_id: "<your_tenant_id>"
     region: "HaNoi" # HaNoi or HoChiMinh
     api_url: "https://manage.bizflycloud.vn"
   ```

3. **Deploy File Storage CSI Plugin**:
   ```bash
   kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-driver.yaml
   kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-controllerplugin-rbac.yaml
   kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-nodeplugin-rbac.yaml
   kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-controllerplugin.yaml
   kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-nodeplugin.yaml
   ```

4. **Create StorageClass**:
   ```yaml
   apiVersion: storage.k8s.io/v1
   kind: StorageClass
   metadata:
     name: bizflycloud-fs
   provisioner: fs.csi.bizflycloud.vn
   parameters:
     zone: "HN1"
     network_id: "<your_vpc_network_id>" # Required
     nfs-shareClient: "10.0.0.0/16"      # Recommended: Your VPC CIDR
   reclaimPolicy: Delete
   volumeBindingMode: Immediate
   allowVolumeExpansion: true
   ```

---

## Block Storage CSI Driver (ReadWriteOnce)

### 1. Create Secret with Credentials

*Note: Skip this step if you have already created the `bizflycloud` secret or installed [Bizfly Cloud Controller Manager](https://github.com/bizflycloud/bizfly-cloud-controller-manager).*

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: bizflycloud
  namespace: kube-system
stringData:
  application_credential_id: "<your_application_credential_id>"
  application_credential_secret: "<your_application_credential_secret>"
  tenant_id: "<your_tenant_id>"
  region: "HaNoi" # HaNoi or HoChiMinh
```

Apply secret:
```bash
kubectl apply -f secret.yaml
```

### 2. Deploy Block Storage CSI Plugin

Select the manifest folder matching your Kubernetes cluster version (e.g. `v1.27`):

```bash
# Set your K8s version tag (v1.20 to v1.27)
K8S_VERSION="v1.27"

# 1. CSI Driver object
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/${K8S_VERSION}/plugin/csi-driver.yaml

# 2. RBAC
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/${K8S_VERSION}/plugin/csi-bizflycloud-controllerplugin-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/${K8S_VERSION}/plugin/csi-bizflycloud-nodeplugin-rbac.yaml

# 3. Controller StatefulSet and Node DaemonSet
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/${K8S_VERSION}/plugin/csi-bizflycloud-controllerplugin.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/${K8S_VERSION}/plugin/csi-bizflycloud-nodeplugin.yaml
```

### 3. Create StorageClass for Block Storage

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: bizflycloud-hdd
provisioner: volume.csi.bizflycloud.vn
parameters:
  type: "HDD"
  availability-zone: "HN1"
reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: true
```

---

## Volume Snapshot (Block Storage)

1. **VolumeSnapshot CRDs** (Kubernetes v1.20+ GA):
   ```bash
   kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/v1.27/v1_volumesnapshot_crds.yaml
   ```

2. **VolumeSnapshotClass**:
   ```yaml
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshotClass
   metadata:
     name: bizflycloud
   driver: volume.csi.bizflycloud.vn
   deletionPolicy: Delete
   parameters:
     force-create: "true"
   ```
