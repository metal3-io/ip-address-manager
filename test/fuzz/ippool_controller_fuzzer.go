package ipam

import (
	"context"
	"fmt"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	controller "github.com/metal3-io/ip-address-manager/controllers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TestCaseM3IPCToM3IPP struct {
	IPClaim       *ipamv1.IPClaim
	ExpectRequest bool
}

func FuzzIPClaimToIPPool(data []byte) int {
	f := fuzz.NewConsumer(data)
	tc := &TestCaseM3IPCToM3IPP{}
	err := f.GenerateStruct(tc)
	if err != nil {
		return 0
	}
	r := controller.IPPoolReconciler{}
	fmt.Printf("tc: %+v\n", tc)

	obj := client.Object(tc.IPClaim)
	reqs := r.IPClaimToIPPool(context.Background(), obj)

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
		} else if req.NamespacedName.Namespace != tc.IPClaim.Spec.Pool.Namespace {
			return 0
		}
	} else if len(reqs) != 0 {
		return 0
	}
	return 1
}
