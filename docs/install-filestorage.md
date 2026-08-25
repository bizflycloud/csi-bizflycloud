# Install Bizfly Cloud File Storage CSI Driver (NFS / ReadWriteMany)

This guide walks you through deploying and configuring the **Bizfly Cloud File Storage CSI Driver** (`fs.csi.bizflycloud.vn`) on your Kubernetes cluster.

Bizfly Cloud File Storage provides scalable, managed NFS-backed shared filesystems with **ReadWriteMany (RWX)** capability, allowing multiple Pods running on different nodes to read and write to the same volume concurrently.

---

## 📋 Prerequisites

Before installing the driver, ensure you have:

1. **Kubernetes Cluster**: Kubernetes v1.20+ (e.g., Bizfly Cloud Managed Kubernetes).
2. **Tools**: `kubectl` and `helm` (v3+) configured to access your cluster.
3. **Bizfly Cloud Credentials & Network Info**:
   - `application_credential_id` & `application_credential_secret` (created from Bizfly Cloud IAM / User Security Settings).
   - `tenant_id` (Project ID).
   - `region` (`HaNoi` or `HoChiMinh`).
   - `network_id` (VPC Network ID where your Kubernetes nodes and File Storage reside — found in Bizfly Cloud Dashboard under **VPC Network**).

> [!NOTE]
> All installation steps are executed directly via `kubectl` and `helm` against the Kubernetes API. No SSH access to worker nodes is required.

---

## Step 1: Install `csi-driver-nfs` (Required Upstream Driver)

The Bizfly Cloud File Storage CSI Driver integrates with the upstream Kubernetes `csi-driver-nfs` for data path mounting on worker nodes. You **must** install `csi-driver-nfs` in your cluster first:

```bash
# Add the csi-driver-nfs Helm repository
helm repo add csi-driver-nfs https://raw.githubusercontent.com/kubernetes-csi/csi-driver-nfs/master/charts
helm repo update

# Install csi-driver-nfs in kube-system namespace
helm install csi-driver-nfs csi-driver-nfs/csi-driver-nfs \
    --namespace kube-system \
    --set feature.enableFSGroupPolicy=true
```

Verify that the `csi-driver-nfs` controller and node pods are running:

```bash
kubectl get pods -n kube-system -l app.kubernetes.io/name=csi-driver-nfs
```

---

## Step 2: Create Bizfly Cloud Authentication Secret

Create the `bizflycloud` secret in the `kube-system` namespace.

> [!TIP]
> If you have already installed the Bizfly Cloud Cloud Controller Manager (CCM) or Block Storage CSI Driver and the `bizflycloud` secret already exists in `kube-system`, ensure it includes the `api_url` field and skip creating a new secret.

Save the following manifest as `secret.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: bizflycloud
  namespace: kube-system
stringData:
  application_credential_id: "<YOUR_APPLICATION_CREDENTIAL_ID>"
  application_credential_secret: "<YOUR_APPLICATION_CREDENTIAL_SECRET>"
  tenant_id: "<YOUR_TENANT_ID>"
  region: "HaNoi" # Options: HaNoi, HoChiMinh
  api_url: "https://manage.bizflycloud.vn"
```

Apply the secret:

```bash
kubectl apply -f secret.yaml
```

---

## Step 3: Deploy Bizfly Cloud File Storage CSI Plugin

Apply the CSI Driver object, RBAC roles, Controller plugin (StatefulSet), and Node plugin (DaemonSet):

```bash
# 1. CSIDriver registration
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-driver.yaml

# 2. RBAC for Controller and Node plugins
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-controllerplugin-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-nodeplugin-rbac.yaml

# 3. Controller StatefulSet and Node DaemonSet
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-controllerplugin.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-nodeplugin.yaml
```

Verify that the File Storage CSI components are running:

```bash
# Check Controller StatefulSet pod
kubectl get pods -n kube-system -l app=csi-fs-bizflycloud-controllerplugin

# Check Node DaemonSet pods (one per worker node)
kubectl get pods -n kube-system -l app=csi-fs-bizflycloud-nodeplugin

# Check registered CSIDrivers
kubectl get csidrivers
```

You should see `fs.csi.bizflycloud.vn` listed under CSIDrivers.

---

## Step 4: Create a StorageClass

Create a `StorageClass` that references `fs.csi.bizflycloud.vn` as the provisioner.

Save the following manifest as `storageclass-fs.yaml`:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: bizflycloud-fs
provisioner: fs.csi.bizflycloud.vn
parameters:
  # Availability zone of your cluster / VPC (HN1 or HCM1)
  zone: "HN1"

  # Share protocol (default: NFS)
  share_protocol: NFS

  # REQUIRED: VPC Network ID from Bizfly Cloud Dashboard
  network_id: "<YOUR_VPC_NETWORK_ID>"

  # OPTIONAL: Subnet ID within your VPC
  # subnet_id: "<YOUR_VPC_SUBNET_ID>"

  # IP Access Rule: CIDR allowed to mount NFS shares
  # Recommended: Set to your VPC CIDR (e.g., 10.0.0.0/16) for security
  nfs-shareClient: "10.0.0.0/16"

reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: true
```

### Parameter Details:

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `zone` | string | Optional | Availability zone (`HN1`, `HCM1`). Defaults to node zone if omitted. |
| `share_protocol` | string | Optional | Shared filesystem protocol. Default is `NFS`. |
| `network_id` | string | **Yes** | The UUID of your Bizfly Cloud VPC Network. |
| `subnet_id` | string | Optional | Specific Subnet UUID within the VPC. |
| `share_type` | string | Optional | File Storage share type (default: `share-type-1`). |
| `nfs-shareClient`| string | Optional | Allowed IP range / CIDR for NFS access. Defaults to `0.0.0.0/0`. Recommended: your VPC CIDR (e.g. `10.0.0.0/16`). |

Apply the StorageClass:

```bash
kubectl apply -f storageclass-fs.yaml
```

---

## Step 5: Test Shared Storage (ReadWriteMany)

### 1. Create a PersistentVolumeClaim (PVC)

Save the following manifest as `pvc-fs.yaml`:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-shared-storage
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  storageClassName: bizflycloud-fs
```

Apply the PVC:

```bash
kubectl apply -f pvc-fs.yaml
```

Check the PVC status:

```bash
kubectl get pvc my-shared-storage
```

The status should become `Bound` once Bizfly Cloud provisions the NFS share.

---

### 2. Deploy Multiple Pods Sharing the Volume

Save the following manifest as `deployment-fs.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fs-shared-test
  labels:
    app: fs-shared-test
spec:
  replicas: 3
  selector:
    matchLabels:
      app: fs-shared-test
  template:
    metadata:
      labels:
        app: fs-shared-test
    spec:
      containers:
      - name: writer
        image: busybox
        command:
          - "sh"
          - "-c"
          - |
            while true; do
              echo "[$(date)] Written by pod $HOSTNAME" >> /mnt/shared/data.txt
              sleep 5
            done
        volumeMounts:
        - name: shared-storage
          mountPath: /mnt/shared
      volumes:
      - name: shared-storage
        persistentVolumeClaim:
          claimName: my-shared-storage
```

Apply the Deployment:

```bash
kubectl apply -f deployment-fs.yaml
```

Verify that all 3 pods are running and writing to the same shared file:

```bash
# Check pods
kubectl get pods -l app=fs-shared-test

# Check shared file content across pods
kubectl exec -it deployment/fs-shared-test -- cat /mnt/shared/data.txt
```

---

## 📈 Volume Expansion (Resizing)

To expand an existing File Storage volume, update the `storage` request in your PVC:

```bash
kubectl patch pvc my-shared-storage -p '{"spec":{"resources":{"requests":{"storage":"20Gi"}}}}'
```

After a few moments, the underlying share will be resized automatically:

```bash
kubectl get pvc my-shared-storage
```

> [!IMPORTANT]
> - Volume size can only be increased, not decreased.
> - Expanding a volume online does not interrupt running workloads.

---

## ❓ Troubleshooting

### 1. Node plugin pod fails with socket connection error
**Cause**: `csi-driver-nfs` is not installed or its socket is not at `/var/lib/kubelet/plugins/csi-nfsplugin/csi.sock`.  
**Fix**: Ensure you ran `helm install csi-driver-nfs ...` as described in Step 1.

### 2. PVC stays in `Pending` state
**Cause**: Missing or incorrect authentication credentials, or invalid `network_id` in StorageClass.  
**Fix**: Check the controller plugin logs:
```bash
kubectl logs -n kube-system -l app=csi-fs-bizflycloud-controllerplugin -c fs-csi-plugin --tail=100
```

### 3. Pod mount hangs or times out
**Cause**: The NFS share IP is not reachable from the worker node or blocked by security rules.  
**Fix**: Verify that `nfs-shareClient` in your StorageClass includes your worker nodes' subnet / VPC CIDR (e.g. `10.0.0.0/16`).
