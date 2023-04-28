package ipam

import (
	"fmt"

	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	controller "github.com/metal3-io/ip-address-manager/controllers"
	ipam "github.com/metal3-io/ip-address-manager/ipam"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

type TestCaseM3IPCToM3IPP struct {
	IPClaim       *ipamv1.IPClaim
	ExpectRequest bool
}

func IPClaimToPool(b []byte) int {
	var tc TestCaseM3IPCToM3IPP
	if err := sigyaml.Unmarshal(b, &tc); err != nil {
		return 0
	}
	r := controller.IPPoolReconciler{}
	obj := client.Object(tc.IPClaim)
	reqs := r.IPClaimToIPPool(obj)

	if tc.ExpectRequest {
		if len(reqs) != 1 {
			return 0
		}

		req := reqs[0]
		if req.NamespacedName.Name != tc.IPClaim.Spec.Pool.Name {
			return 0
		}

		if tc.IPClaim.Spec.Pool.Namespace == "" && req.NamespacedName.Namespace != tc.IPClaim.Namespace {
			return 0

		} else {
			if req.NamespacedName.Namespace != tc.IPClaim.Spec.Pool.Namespace {
				return 0
			}
		}

	} else {
		if len(reqs) != 0 {
			return 0
		}

	}
	return 1
}
