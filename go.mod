module github.com/metal3-io/ip-address-manager

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.5.0
	github.com/google/uuid v1.1.4 // indirect
	github.com/metal3-io/ip-address-manager/api v0.0.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/pkg/errors v0.9.1
	k8s.io/api v0.22.2
	k8s.io/apiextensions-apiserver v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/cluster-api v0.4.3
	sigs.k8s.io/controller-runtime v0.10.1
)

replace github.com/metal3-io/ip-address-manager/api => ./api
