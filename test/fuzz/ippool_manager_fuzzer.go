package ipam

import (
	"log"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	ipam "github.com/metal3-io/ip-address-manager/ipam"
)

func FuzzNewIPPoolManager(data []byte) int {
	f := fuzz.NewConsumer(data)
	ipPool := &ipamv1.IPPool{}

	err := f.GenerateStruct(ipPool)
	if err != nil {
		return 0
	}

	log.Printf("before %+v\n", ipPool)

	ipPoolMgr, err := ipam.NewIPPoolManager(nil, ipPool,
		logr.Discard(),
	)
	if err != nil {
		return 0
	}

	ipPoolMgr.SetFinalizer()
	log.Printf("after %+v\n", ipPool)

	if len(ipPool.ObjectMeta.Finalizers) == 0 {
		return 0
	}

	ipPoolMgr.UnsetFinalizer()

	if len(ipPool.ObjectMeta.Finalizers) != 0 {
		return 0
	}
	return 1
}
