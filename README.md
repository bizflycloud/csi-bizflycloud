# Bizfly Cloud CSI Drivers for Kubernetes

A set of Container Storage Interface (CSI) Drivers for Bizfly Cloud Storage products on Kubernetes.

---

## 📖 Overview

This repository allows Kubernetes clusters to dynamically provision, attach, and manage Bizfly Cloud storage volumes through two distinct CSI plugins:

| Driver Name | Provisioner | Access Modes | Protocol | Common Use Cases |
| :--- | :--- | :--- | :--- | :--- |
| **Bizfly Cloud File Storage** | `fs.csi.bizflycloud.vn` | **ReadWriteMany (RWX)**, ReadWriteOnce (RWO) | NFS | Multi-replica web servers, CMS (WordPress), shared data, ML pipelines, shared logs |
| **Bizfly Cloud Block Storage** | `volume.csi.bizflycloud.vn` | **ReadWriteOnce (RWO)** | Block Device | High-performance databases (MySQL, Postgres), Kafka, Elasticsearch |

---

## 🚀 Installation

> [!NOTE]
> All installation steps are executed directly from your local machine / CI pipeline using `kubectl` and `helm` against your Kubernetes cluster. No SSH access to worker nodes is required.

---

### 📂 Option 1: Install File Storage CSI Driver (ReadWriteMany / RWX)

Bizfly Cloud File Storage provides scalable, managed NFS-backed shared filesystems where multiple Pods across different nodes can read and write simultaneously.

#### Step 1: Install upstream `csi-driver-nfs` (Required)

The Bizfly Cloud File Storage driver leverages the standard Kubernetes `csi-driver-nfs` on worker nodes for mount operations. Deploy it via Helm:

```bash
helm repo add csi-driver-nfs https://raw.githubusercontent.com/kubernetes-csi/csi-driver-nfs/master/charts
helm repo update

helm install csi-driver-nfs csi-driver-nfs/csi-driver-nfs \
    --namespace kube-system \
    --set feature.enableFSGroupPolicy=true
```

Verify that `csi-driver-nfs` pods are running:
```bash
kubectl get pods -n kube-system -l app.kubernetes.io/name=csi-driver-nfs
```

#### Step 2: Create Bizfly Cloud Authentication Secret

Create the `bizflycloud` secret in the `kube-system` namespace with your credentials from the Bizfly Cloud Dashboard (IAM / User Security):

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

Save as `secret.yaml` and apply:
```bash
kubectl apply -f secret.yaml
```

#### Step 3: Deploy File Storage CSI Plugin Manifests

```bash
# 1. CSI Driver object
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-driver.yaml

# 2. RBAC for Controller and Node plugins
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-controllerplugin-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-nodeplugin-rbac.yaml

# 3. Controller StatefulSet and Node DaemonSet
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-controllerplugin.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-nodeplugin.yaml
```

Verify installation:
```bash
kubectl get pods -n kube-system -l app=csi-fs-bizflycloud-controllerplugin
kubectl get pods -n kube-system -l app=csi-fs-bizflycloud-nodeplugin
kubectl get csidriver fs.csi.bizflycloud.vn
```

#### Step 4: Create StorageClass

Create a `StorageClass` pointing to `fs.csi.bizflycloud.vn`. Replace `<your_vpc_network_id>` with your VPC Network ID (found on Bizfly Cloud Dashboard under **VPC Network**):

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

  # Required: ID of your VPC Network in Bizfly Cloud
  network_id: "<your_vpc_network_id>"

  # IP access rule: CIDR allowed to mount NFS shares (Recommended: Your VPC CIDR)
  nfs-shareClient: "10.0.0.0/16"

reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: true
```

Apply:
```bash
kubectl apply -f storageclass.yaml
```

#### Step 5: Test Shared Storage (RWX)

Create a PVC with `ReadWriteMany` access mode:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: shared-pvc
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  storageClassName: bizflycloud-fs
```

Deploy a 3-replica Deployment sharing the same volume:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shared-test-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: shared-test
  template:
    metadata:
      labels:
        app: shared-test
    spec:
      containers:
      - name: writer
        image: busybox
        command: ["sh", "-c", "while true; do echo \"[$(date)] Written by pod $HOSTNAME\" >> /mnt/shared/log.txt; sleep 5; done"]
        volumeMounts:
        - name: shared-vol
          mountPath: /mnt/shared
      volumes:
      - name: shared-vol
        persistentVolumeClaim:
          claimName: shared-pvc
```

Verify shared write across pods:
```bash
kubectl exec -it deployment/shared-test-app -- cat /mnt/shared/log.txt
```

👉 See full guide: **[docs/install-filestorage.md](docs/install-filestorage.md)**.

---

### 💾 Option 2: Install Block Storage CSI Driver (ReadWriteOnce / RWO)

#### Step 1: Create Secret

If not already created, apply the `bizflycloud` secret as shown above.

#### Step 2: Deploy Block Storage CSI Plugin

Select the folder corresponding to your Kubernetes version (e.g. `v1.27`):

```bash
K8S_VERSION="v1.27"

kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/${K8S_VERSION}/plugin/csi-driver.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/${K8S_VERSION}/plugin/csi-bizflycloud-controllerplugin-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/${K8S_VERSION}/plugin/csi-bizflycloud-nodeplugin-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/${K8S_VERSION}/plugin/csi-bizflycloud-controllerplugin.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/${K8S_VERSION}/plugin/csi-bizflycloud-nodeplugin.yaml
```

#### Step 3: Create StorageClass

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

👉 See full guide: **[docs/install.md](docs/install.md)**.

---

## ⚡ Features & Capabilities

### Volume Expansion (Resizing)

Both File Storage and Block Storage drivers support online volume expansion by increasing the `storage` request on the PVC:

```bash
kubectl patch pvc shared-pvc -p '{"spec":{"resources":{"requests":{"storage":"20Gi"}}}}'
```

*Note: Volume sizes can only be increased, not decreased.*

### Raw Block Volume (Block Storage)

Block Storage supports raw block device mode by setting `volumeMode: Block` on the PVC:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: block-pvc
spec:
  accessModes:
    - ReadWriteOnce
  volumeMode: Block
  resources:
    requests:
      storage: 20Gi
  storageClassName: bizflycloud-hdd
```

### Volume Snapshots (Block Storage)

Snapshots can be created and restored through Kubernetes `VolumeSnapshot` objects. See [docs/install.md#volume-snapshot](docs/install.md#volume-snapshot-block-storage).

---

## 📁 Repository Layout

```
.
├── cmd/
│   ├── csi-bizflycloud/      # Block Storage CSI Driver main
│   └── csi-fs-bizflycloud/   # File Storage CSI Driver main
├── driver/
│   ├── blockstorage/         # Block Storage CSI implementation
│   └── filestorage/          # File Storage CSI implementation
├── manifest/
│   ├── fs-bizflycloud/       # File Storage manifests
│   └── v1.20 - v1.27/        # Block Storage manifests by K8s version
├── examples/
│   ├── fs-bizflycloud/       # File Storage StorageClass, PVC & Deployment examples
│   └── ...                   # Block Storage examples
└── docs/
    ├── install.md            # CSI Drivers installation overview
    └── install-filestorage.md# Dedicated File Storage guide
```

---

## 🤝 Contributing

At Bizfly Cloud we value and love our community! If you have any issues or would like to contribute, feel free to open an issue or PR.
