module github.com/metal3-io/ip-address-manager

go 1.16

require (
	cloud.google.com/go v0.75.0 // indirect
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.4.4
	github.com/google/uuid v1.1.4 // indirect
	github.com/metal3-io/ip-address-manager/api v0.0.0
	github.com/onsi/ginkgo v1.16.1
	github.com/onsi/gomega v1.11.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/procfs v0.3.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/text v0.3.5 // indirect
	k8s.io/api v0.21.0
	k8s.io/apiextensions-apiserver v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.21.0
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10
	sigs.k8s.io/cluster-api v0.3.11-0.20210429135050-9d825629d89c
	sigs.k8s.io/controller-runtime v0.9.0-beta.0
)

replace github.com/metal3-io/ip-address-manager/api => ./api
