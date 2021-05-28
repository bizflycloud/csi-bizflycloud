# Install BizFly Cloud CSI Driver

## 1. Create secret with your credentials

*Note: Skip this step if you are already installed [BizFly Cloud Controller Manager](https://github.com/bizflycloud/bizfly-cloud-controller-manager)*

Replace the credentials in the `manifest/secret.yaml` as bellow:

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
```

Create Secret in `kube-system` namespace

```shell script
kubectl apply -f manifest/secret.yaml
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

For kubernetes >= v1.17 and <= v1.19, [Beta Volume Snapshot](https://kubernetes.io/blog/2019/12/09/kubernetes-1-17-feature-cis-volume-snapshot-beta/) need manually installed.

```shell script
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/v1beta1_volumesnapshot_crds.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/examples/v1beta1_volumesnapshot/v1beta1_volumesnapshotclasses.yaml
```

See also [the example](/examples/v1beta1_volumesnapshot).
