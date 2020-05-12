module github.com/metal3-io/ipam

go 1.13

require (
	4d63.com/gochecknoglobals v0.0.0-20190306162314-7c3491d2b6ec // indirect
	4d63.com/gochecknoinits v0.0.0-20200108094044-eb73b47b9fc4 // indirect
	cloud.google.com/go v0.56.0 // indirect
	github.com/alecthomas/gocyclo v0.0.0-20150208221726-aa8f8b160214 // indirect
	github.com/alexkohler/nakedret v1.0.0 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/swag v0.19.9 // indirect
	github.com/golang/mock v1.4.3
	github.com/golang/protobuf v1.4.0 // indirect
	github.com/google/gofuzz v1.1.0
	github.com/gordonklaus/ineffassign v0.0.0-20200309095847-7953dde2c7bf // indirect
	github.com/imdario/mergo v0.3.9 // indirect
	github.com/jgautheron/goconst v0.0.0-20200227150835-cda7ea3bf591 // indirect
	github.com/mdempsky/unconvert v0.0.0-20200228143138-95ecdbfc0b5f // indirect
	github.com/metal3-io/baremetal-operator v0.0.0-20200424085833-a1dd8aca186d
	github.com/metal3-io/cluster-api-provider-metal3 v0.3.1
	github.com/mibk/dupl v1.0.0 // indirect
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/opennota/check v0.0.0-20180911053232-0c771f5545ff // indirect
	github.com/pkg/errors v0.9.1
	github.com/stripe/safesql v0.2.0 // indirect
	github.com/tsenart/deadcode v0.0.0-20160724212837-210d2dc333e9 // indirect
	github.com/walle/lll v1.0.1 // indirect
	golang.org/x/crypto v0.0.0-20200423211502-4bdfaf469ed5 // indirect
	golang.org/x/net v0.0.0-20200425230154-ff2c4b7c35a0
	golang.org/x/sys v0.0.0-20200420163511-1957bb5e6d1f // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	google.golang.org/appengine v1.6.6 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.19.0-alpha.2
	k8s.io/apiextensions-apiserver v0.18.2
	k8s.io/apimachinery v0.19.0-alpha.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/cluster-bootstrap v0.18.2 // indirect
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200413232311-afe0b5e9f729 // indirect
	k8s.io/utils v0.0.0-20200414100711-2df71ebbae66
	mvdan.cc/interfacer v0.0.0-20180901003855-c20040233aed // indirect
	mvdan.cc/lint v0.0.0-20170908181259-adc824a0674b // indirect
	mvdan.cc/unparam v0.0.0-20200314162735-0ac8026f7d06 // indirect
	sigs.k8s.io/cluster-api v0.3.3
	sigs.k8s.io/controller-runtime v0.6.0
	sigs.k8s.io/kind v0.8.0 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace (
	k8s.io/api => k8s.io/api v0.19.0-alpha.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.0-alpha.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.0-alpha.2
	k8s.io/apiserver => k8s.io/apiserver v0.19.0-alpha.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.0-alpha.2
	k8s.io/client-go => k8s.io/client-go v0.19.0-alpha.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.0-alpha.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.0-alpha.2
	k8s.io/code-generator => k8s.io/code-generator v0.19.0-alpha.2
	k8s.io/component-base => k8s.io/component-base v0.19.0-alpha.2
	k8s.io/cri-api => k8s.io/cri-api v0.19.0-alpha.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.0-alpha.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.0-alpha.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.0-alpha.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.0-alpha.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.0-alpha.2
	k8s.io/kubectl => k8s.io/kubectl v0.19.0-alpha.2
	k8s.io/kubelet => k8s.io/kubelet v0.19.0-alpha.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.0-alpha.2
	k8s.io/metrics => k8s.io/metrics v0.19.0-alpha.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.0-alpha.2
) // Required by BMO

replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by BMO

replace github.com/openshift/api => github.com/openshift/api v0.0.0-20190924102528-32369d4db2ad // Required by BMO until https://github.com/operator-framework/operator-lifecycle-manager/pull/1241 is resolved

//replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.0 // Issue with go-client version
