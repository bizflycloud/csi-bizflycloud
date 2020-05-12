module github.com/bizflycloud/csi-bizflycloud

go 1.14

replace k8s.io/api => k8s.io/api v0.18.2

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.2

replace k8s.io/apimachinery => k8s.io/apimachinery v0.18.3-beta.0

replace k8s.io/apiserver => k8s.io/apiserver v0.18.2

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.2

replace k8s.io/client-go => k8s.io/client-go v0.18.2

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.2

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.2

replace k8s.io/code-generator => k8s.io/code-generator v0.18.3-beta.0

replace k8s.io/component-base => k8s.io/component-base v0.18.2

replace k8s.io/cri-api => k8s.io/cri-api v0.18.3-beta.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.2

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.2

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.2

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.2

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.2

replace k8s.io/kubectl => k8s.io/kubectl v0.18.2

replace k8s.io/kubelet => k8s.io/kubelet v0.18.2

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.2

replace k8s.io/metrics => k8s.io/metrics v0.18.2

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.2

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.18.2

replace k8s.io/sample-controller => k8s.io/sample-controller v0.18.2

require (
	github.com/bizflycloud/gobizfly v0.0.0-20200509022858-1dd705ff35b3
	github.com/container-storage-interface/spec v1.2.0
	github.com/golang/protobuf v1.4.1 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	golang.org/x/net v0.0.0-20200506145744-7e3656a0809f
	golang.org/x/sys v0.0.0-20200501145240-bc7a7d42d5c3 // indirect
	google.golang.org/genproto v0.0.0-20200507105951-43844f6eee31 // indirect
	google.golang.org/grpc v1.29.1
	k8s.io/apimachinery v0.18.2
	k8s.io/cloud-provider-openstack v1.18.0
	k8s.io/component-base v0.18.2
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.18.2 // indirect
	k8s.io/utils v0.0.0-20200414100711-2df71ebbae66
)
