module github.com/NVIDIA/k8s-device-plugin

go 1.14

replace (
	k8s.io/api => k8s.io/api v0.19.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.1
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.1
	k8s.io/apiserver => k8s.io/apiserver v0.19.1
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.1
	k8s.io/client-go => k8s.io/client-go v0.19.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.1
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.1
	k8s.io/code-generator => k8s.io/code-generator v0.19.1
	k8s.io/component-base => k8s.io/component-base v0.19.1
	k8s.io/cri-api => k8s.io/cri-api v0.19.1
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.1
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.1
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.1
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.1
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.1
	k8s.io/kubectl => k8s.io/kubectl v0.19.1
	k8s.io/kubelet => k8s.io/kubelet v0.19.1
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.1
	k8s.io/metrics => k8s.io/metrics v0.19.1
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.1
)

require (
	github.com/NVIDIA/go-gpuallocator v0.2.0
	github.com/NVIDIA/gpu-monitoring-tools v0.0.0-20200922190416-1e3b4dbd36b2
	github.com/fsnotify/fsnotify v1.4.9
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	google.golang.org/grpc v1.29.0
	k8s.io/kubelet v0.0.0
)
