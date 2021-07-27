module github.com/metal3-io/ip-address-manager

go 1.16

require (
	cloud.google.com/go v0.75.0 // indirect
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.4.4
	github.com/google/uuid v1.1.4 // indirect
	github.com/metal3-io/ip-address-manager/api v0.0.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	k8s.io/api v0.21.1
	k8s.io/apiextensions-apiserver v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/cluster-api v0.4.0-beta.0
	sigs.k8s.io/controller-runtime v0.9.0
)

replace github.com/metal3-io/ip-address-manager/api => ./api
