# Install BizFly Cloud CSI Driver

## 1. Create secret with your credentials

*Note: Skip this step if you are already installed [BizFly Cloud Controller Manager](https://github.com/bizflycloud/bizfly-cloud-controller-manager)*

Replace the credentials in the `manifest/plugin/secret.yaml` as bellow:

 ```yaml
apiVersion: v1
kind: Secret
metadata:
  name: bizflycloud
  namespace: kube-system
stringData:
  application_credential_id: "your_application_credential_id"
  application_credential_secret: "your_application_credential_secret"
  tenant_id: "your_tenant_id"
  region: "your_region" # HN, HCM
```

Create Secret in `kube-system` namespace

```shell script
kubectl apply -f manifest/plugin/secret.yaml
```

## 2. Deploy the CSI Plugin

- Create a new CSI Driver

```shell script
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/plugin/csi-driver.yaml
```

- Add RBAC for `controller plugin` and `node plugin`

```shell script
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/plugin/csi-bizflycloud-controllerplugin-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/plugin/csi-bizflycloud-nodeplugin-rbac.yaml
```

- Install CSI Statefulset and Daemonset 

```shell script
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/plugin/csi-bizflycloud-controllerplugin.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/plugin/csi-bizflycloud-nodeplugin.yaml
```

# Volume Snapshot

1. CRDs is required to enable VolumeSnapshot

[Beta VolumeSnapshot](https://kubernetes.io/blog/2019/12/09/kubernetes-1-17-feature-cis-volume-snapshot-beta/) since Kubernetes v1.17

```shell script
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/v1beta1_volumesnapshot_crds.yaml
```

[GA VolumeSnapshot](https://kubernetes.io/blog/2020/12/10/kubernetes-1.20-volume-snapshot-moves-to-ga/) since Kubernetes v1.20

```shell script
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/v1_volumesnapshot_crds.yaml
```

2. VolumeSnapshotClass

```YAML
# apiVersion: snapshot.storage.k8s.io/v1beta1
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: bizflycloud
driver: volume.csi.bizflycloud.vn
deletionPolicy: Delete
parameters:
  force-create: "true"

```
