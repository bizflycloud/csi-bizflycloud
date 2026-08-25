# Hướng dẫn cài đặt Bizfly Cloud File Storage CSI Driver (NFS / ReadWriteMany)

Tài liệu này hướng dẫn chi tiết từng bước để cài đặt, cấu hình và sử dụng **Bizfly Cloud File Storage CSI Driver** (`fs.csi.bizflycloud.vn`) trên cụm Kubernetes (BKE hoặc Kubernetes tự dựng trên hạ tầng Bizfly Cloud).

File Storage của Bizfly Cloud cung cấp hệ thống lưu trữ tệp chia sẻ qua giao thức **NFS** với chế độ truy cập **ReadWriteMany (RWX)**, cho phép nhiều Pod chạy trên các worker node khác nhau có thể đồng thời đọc và ghi vào cùng một Volume.

---

## 📋 Mục lục
1. [Yêu cầu chuẩn bị](#1-yêu-cầu-chuẩn-bị)
2. [Bước 1: Cài đặt Upstream NFS CSI Driver (`csi-driver-nfs`)](#bước-1-cài-đặt-upstream-nfs-csi-driver-csi-driver-nfs)
3. [Bước 2: Tạo Secret xác thực Bizfly Cloud](#bước-2-tạo-secret-xác-thực-bizfly-cloud)
4. [Bước 3: Triển khai File Storage CSI Plugin](#bước-3-triển-khai-file-storage-csi-plugin)
5. [Bước 4: Tạo StorageClass](#bước-4-tạo-storageclass)
6. [Bước 5: Kiểm tra hoạt động với Nginx (RWX)](#bước-5-kiểm-tra-hoạt-động-với-nginx-rwx)
7. [Mở rộng dung lượng Volume (Volume Expansion)](#mở-rộng-dung-lượng-volume-volume-expansion)
8. [Xử lý sự cố thường gặp (Troubleshooting)](#xử-lý-sự-cố-thường-gặp-troubleshooting)

---

## 1. Yêu cầu chuẩn bị

Trước khi cài đặt, bạn cần chuẩn bị:

1. **Cụm Kubernetes**: Phiên bản Kubernetes v1.20+ (ví dụ: Bizfly Kubernetes Engine - BKE).
2. **Công cụ dòng lệnh**: `kubectl` và `helm` (v3+) đã kết nối tới cụm Kubernetes qua `kubeconfig`.
3. **Thông tin xác thực & Mạng từ Bizfly Cloud**:
   - `application_credential_id` & `application_credential_secret`: Tạo tại mục **IAM / Quản lý tài khoản / Khóa truy cập ứng dụng**.
   - `tenant_id`: ID Dự án (Project ID) của bạn trên Bizfly Cloud.
   - `region`: `HaNoi` hoặc `HoChiMinh`.
   - `network_id` & `subnet_id`: ID Mạng VPC và Subnet nơi các worker node Kubernetes đang chạy (có thể xem trong Dashboard mục **VPC Network** hoặc lấy từ nhãn của node bằng lệnh `kubectl get node -o yaml | grep vpc`).
   - `zone`: Availability Zone của cụm (`HN1`, `HN2`, `HCM1`,...).

> [!NOTE]
> Tất cả các bước cài đặt được thực hiện trực tiếp thông qua `kubectl` và `helm`. Không yêu cầu quyền SSH vào các worker node.

---

## Bước 1: Cài đặt Upstream NFS CSI Driver (`csi-driver-nfs`)

Bizfly Cloud File Storage CSI Driver tích hợp với upstream `csi-driver-nfs` để thực hiện thao tác mount NFS trên các worker node. Bạn **bắt buộc** phải cài đặt driver này trước:

```bash
# 1. Thêm Helm repository csi-driver-nfs
helm repo add csi-driver-nfs https://raw.githubusercontent.com/kubernetes-csi/csi-driver-nfs/master/charts
helm repo update

# 2. Cài đặt vào namespace kube-system
helm install csi-driver-nfs csi-driver-nfs/csi-driver-nfs \
    --namespace kube-system \
    --set feature.enableFSGroupPolicy=true
```

Kiểm tra trạng thái các Pod của NFS driver:
```bash
kubectl get pods -n kube-system -l app.kubernetes.io/name=csi-driver-nfs
```
*Tất cả các pod `csi-nfs-controller` và `csi-nfs-node` phải ở trạng thái **Running**.*

---

## Bước 2: Tạo Secret xác thực Bizfly Cloud

Tạo Secret `bizflycloud` trong namespace `kube-system`:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: bizflycloud
  namespace: kube-system
stringData:
  application_credential_id: "<YOUR_APPLICATION_CREDENTIAL_ID>"
  application_credential_secret: "<YOUR_APPLICATION_CREDENTIAL_SECRET>"
  tenant_id: "<YOUR_TENANT_ID>"
  region: "HaNoi" # HaNoi hoặc HoChiMinh
  api_url: "https://manage.bizflycloud.vn"
EOF
```

> [!TIP]
> Nếu cụm Kubernetes của bạn đã cài đặt Bizfly Cloud Controller Manager (CCM) hoặc Block Storage CSI Driver và Secret `bizflycloud` đã tồn tại, hãy kiểm tra xem Secret đó đã có đủ trường `api_url` chưa và có thể tái sử dụng.

---

## Bước 3: Triển khai File Storage CSI Plugin

Áp dụng các manifest định nghĩa CSI Driver, RBAC, StatefulSet Controller và DaemonSet Node Plugin:

```bash
# 1. Đăng ký CSIDriver fs.csi.bizflycloud.vn
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-driver.yaml

# 2. Tạo RBAC cho Controller và Node Plugin
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-controllerplugin-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-nodeplugin-rbac.yaml

# 3. Triển khai Controller (StatefulSet) và Node (DaemonSet)
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-controllerplugin.yaml
kubectl apply -f https://raw.githubusercontent.com/bizflycloud/csi-bizflycloud/master/manifest/fs-bizflycloud/plugin/csi-fs-bizflycloud-nodeplugin.yaml
```

Kiểm tra trạng thái các thành phần:
```bash
# Kiểm tra Controller Plugin pod (phải đạt 3/3 Running)
kubectl get pods -n kube-system -l app=csi-fs-bizflycloud-controllerplugin

# Kiểm tra Node Plugin DaemonSet (phải có 1 pod 2/2 Running trên mỗi worker node)
kubectl get pods -n kube-system -l app=csi-fs-bizflycloud-nodeplugin

# Kiểm tra danh sách CSIDriver đã đăng ký
kubectl get csidrivers
```

---

## Bước 4: Tạo StorageClass

Tạo file `storageclass-fs.yaml` với các thông số tương ứng với hạ tầng mạng VPC của bạn:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: bizflycloud-fs
provisioner: fs.csi.bizflycloud.vn
parameters:
  # Availability Zone của VPC / Node (ví dụ: HN1, HN2, HCM1)
  zone: "HN2"

  # Giao thức chia sẻ (mặc định: NFS)
  share_protocol: NFS

  # BẮT BUỘC: ID của VPC Network từ Bizfly Cloud Dashboard
  network_id: "<YOUR_VPC_NETWORK_ID>"

  # TÙY CHỌN: ID của Subnet trong VPC
  subnet_id: "<YOUR_VPC_SUBNET_ID>"

  # Dải IP CIDR được phép mount NFS (Khuyến nghị: Dải CIDR của VPC hoặc Subnet)
  nfs-shareClient: "10.0.0.0/8"

reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: true
```

Áp dụng StorageClass:
```bash
kubectl apply -f storageclass-fs.yaml
```

---

## Bước 5: Kiểm tra hoạt động với Nginx (RWX)

### 1. Tạo PersistentVolumeClaim (RWX)

Tạo file `pvc-fs.yaml`:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nginx-shared-pvc
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  storageClassName: bizflycloud-fs
```

Áp dụng và kiểm tra trạng thái:
```bash
kubectl apply -f pvc-fs.yaml
kubectl get pvc nginx-shared-pvc
```
*Khi Bizfly Cloud tạo xong File Storage share, PVC sẽ tự động chuyển sang trạng thái `Bound`.*

---

### 2. Triển khai Nginx Deployment với nhiều Pods chia sẻ Volume

Tạo file `deployment-nginx.yaml`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-rwx-test
  labels:
    app: nginx-rwx-test
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx-rwx-test
  template:
    metadata:
      labels:
        app: nginx-rwx-test
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - nginx-rwx-test
              topologyKey: kubernetes.io/hostname
      containers:
      # Container web server Nginx đọc file index.html từ shared volume
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
        volumeMounts:
        - name: shared-html
          mountPath: /usr/share/nginx/html
      # Container sidecar liên tục ghi log vào shared volume
      - name: writer
        image: busybox
        command:
          - "sh"
          - "-c"
          - |
            while true; do
              echo "<h1>Bizfly FileStorage RWX</h1><p>[$(date)] Pod $HOSTNAME ($NODE_NAME) is writing.</p>" >> /usr/share/nginx/html/index.html
              echo "[$(date)] Pod: $HOSTNAME ($NODE_NAME)" >> /usr/share/nginx/html/access.log
              sleep 5
            done
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        volumeMounts:
        - name: shared-html
          mountPath: /usr/share/nginx/html
      volumes:
      - name: shared-html
        persistentVolumeClaim:
          claimName: nginx-shared-pvc
```

Áp dụng Deployment:
```bash
kubectl apply -f deployment-nginx.yaml
```

---

### 3. Xác thực dữ liệu đọc/ghi đồng thời

Kiểm tra danh sách Pod đang chạy trên các worker node khác nhau:
```bash
kubectl get pods -l app=nginx-rwx-test -o wide
```

Xem nội dung log được ghi đồng thời bởi cả 2 Pod từ các worker node khác nhau:
```bash
kubectl exec -it deployment/nginx-rwx-test -c nginx -- cat /usr/share/nginx/html/access.log
```

---

## Mở rộng dung lượng Volume (Volume Expansion)

File Storage CSI Driver hỗ trợ tăng dung lượng volume online mà không làm gián đoạn workload:

```bash
kubectl patch pvc nginx-shared-pvc -p '{"spec":{"resources":{"requests":{"storage":"20Gi"}}}}'
```

Kiểm tra lại dung lượng:
```bash
kubectl get pvc nginx-shared-pvc
```

> [!IMPORTANT]
> - Dung lượng chỉ có thể tăng lên, không thể giảm xuống.
> - Quá trình mở rộng được thực hiện tự động và giữ nguyên toàn vẹn dữ liệu.

---

## Xử lý sự cố thường gặp (Troubleshooting)

### 1. PVC ở trạng thái `Pending` kéo dài
- **Nguyên nhân**: Thông tin xác thực trong Secret `bizflycloud` không chính xác, hoặc `network_id` trong StorageClass không tồn tại / không đúng định dạng UUID.
- **Cách kiểm tra**:
  ```bash
  # Xem log Controller Provisioner
  kubectl logs -n kube-system -l app=csi-fs-bizflycloud-controllerplugin -c csi-provisioner --tail=50

  # Xem log Plugin gọi API Bizfly Cloud
  kubectl logs -n kube-system -l app=csi-fs-bizflycloud-controllerplugin -c fs-csi-plugin --tail=50
  ```

### 2. Pod báo lỗi `mount.nfs: access denied by server`
- **Nguyên nhân**: Tham số `nfs-shareClient` trong StorageClass chưa bao gồm dải mạng IP của worker node, hoặc worker node không thể định tuyến tới IP của share NFS.
- **Cách kiểm tra & khắc phục**:
  - Đảm bảo `nfs-shareClient` trong StorageClass được đặt là dải CIDR của VPC chứa cluster (ví dụ: `10.20.0.0/16` hoặc `10.0.0.0/8`).
  - Kiểm tra xem Security Group của VPC có cho phép traffic NFS (port 2049) nội bộ trong mạng VPC hay không.

### 3. Node plugin báo lỗi kết nối socket NFS
- **Nguyên nhân**: Chưa cài đặt `csi-driver-nfs` qua Helm ở Bước 1.
- **Khắc phục**: Chạy lệnh Helm install `csi-driver-nfs` như hướng dẫn tại Bước 1.
