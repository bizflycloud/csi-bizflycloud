# BizFly Cloud Volume CSI driver for Kubernetes

A Container Storage Interface (CSI) Driver for BizFlyCloud Block Storage

## About 

This driver allows Kubernetes to use BizFly Volume, csi plugin name `volume.csi.bizflycloud.vn`

## Features

Below is a list of functionality implemented by the plugin. In general, [CSI features](https://kubernetes-csi.github.io/docs/features.html) implementing an aspect of the [specification](https://github.com/container-storage-interface/spec/blob/master/spec.md) are available on any BizFly Cloud Kubernetes version for which beta support for the feature is provided.

See also the [project examples](/examples) for use cases.

### Volume Expansion

Volumes can be expanded by updating the storage request value of the corresponding PVC:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-pvc
  namespace: default
spec:
  [...]
  resources:
    requests:
      # The field below can be increased.
      storage: 20Gi
      [...]
```

After successful expansion, the _status_ section of the PVC object will reflect the actual volume capacity.

Important notes:

* Volumes can only be increased in size, not decreased; attempts to do so will lead to an error.
* Expanding a volume that is larger than the target size will have no effect. The PVC object status section will continue to represent the actual volume capacity.
* Resizing volumes other than through the PVC object (e.g., the BizFly cloud control panel) is not recommended as this can potentially cause conflicts. Additionally, size updates will not be reflected in the PVC object status section immediately, and the section will eventually show the actual volume capacity.

### Raw Block Volume

Volumes can be used in raw block device mode by setting the `volumeMode` on the corresponding PVC:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-pvc
  namespace: default
spec:
  [...]
  volumeMode: Block
```

Important notes:

* If using volume expansion functionality, only expansion of the underlying persistent volume is guaranteed. We do not guarantee to automatically
expand the filesystem if you have formatted the device.

### Volume Snapshots

Snapshots can be created and restored through `VolumeSnapshot` objects.

See also [Volume Snapshot](/docs/install.md#volume-snapshot).

## Install BizFly Cloud CSI Driver

Please Refer to install [BizFly Cloud CSI Driver](/docs/install.md)


## Contributing

At BizFly Cloud we value and love our community! If you have any issues or would like to contribute, feel free to open an issue or PR.

