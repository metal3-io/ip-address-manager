module github.com/metal3-io/ip-address-manager

go 1.16

require (
	github.com/go-logr/logr v1.2.0
	github.com/golang/mock v1.6.0
	github.com/metal3-io/ip-address-manager/api v0.0.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/pkg/errors v0.9.1
	k8s.io/api v0.23.0
	k8s.io/apiextensions-apiserver v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/klog/v2 v2.30.0
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/cluster-api v1.0.2
	sigs.k8s.io/controller-runtime v0.11.0
)

require github.com/google/uuid v1.1.4 // indirect

replace github.com/metal3-io/ip-address-manager/api => ./api
