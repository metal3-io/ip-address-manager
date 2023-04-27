package ipam

import (
	"fmt"

	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	ipam "github.com/metal3-io/ip-address-manager/ipam"
	sigyaml "sigs.k8s.io/yaml"
)

func IPPoolFinalizer(b []byte) int {

	var ipPool *ipamv1.IPPool
	if err := sigyaml.Unmarshal(b, &ipPool); err != nil {
		return 0
	}
	fmt.Print(b)
	fmt.Print("\n")
	fmt.Printf("%+v\n", ipPool)
	if ipPool == nil {
		return 0
	}

	ipPoolMgr, err := ipam.NewIPPoolManager(nil, ipPool,
		logr.Discard(),
	)
	if err != nil {
		return 0
	}

	ipPoolMgr.SetFinalizer()
	if len(ipamv1.IPPoolFinalizer) == 0 {
		return 0
	}

	ipPoolMgr.UnsetFinalizer()

	if len(ipamv1.IPPoolFinalizer) != 0 {
		return 0
	}
	return 1
}
