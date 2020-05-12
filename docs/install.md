# Install BizFly Cloud CSI Driver

## 1. Create secret with your email and password

*Note: Skip this step if you are already installed [BizFly Cloud Controller Manager](https://github.com/bizflycloud/bizfly-cloud-controller-manager)*

Replace the email and password in the `manifest/secret.yaml` as bellow:

 ```yaml
apiVersion: v1
kind: Secret
metadata:
  name: bizflycloud
  namespace: kube-system
stringData:
  email: "youremail@example.com"
  password: "yourPassWORD"
```

Create Secret in `kube-system` namespace

```shell script
kubectl apply -f manifest/secret.yaml
```

## 2. Deploy the CSI Plugin

- Create a new CSI Driver

```shell script
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/csi-bizfly-driver.yaml
```

- Add rold binding for `controller plugin` and `node plugin`

```shell script
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/bizfly-csi-controllerplugin-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/bizfly-csi-nodeplugin-rbac.yaml
```

- Install CSI Statefulset and Daemonset 

```shell script
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/bizfly-csi-controllerplugin.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/bizfly-csi-nodeplugin.yaml
```