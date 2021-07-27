/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/metal3-io/ip-address-manager/ipam"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	ipPoolControllerName = "IPPool-controller"
	requeueAfter         = time.Second * 30
)

// IPPoolReconciler reconciles a IPPool object
type IPPoolReconciler struct {
	Client           client.Client
	ManagerFactory   ipam.ManagerFactoryInterface
	Log              logr.Logger
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ippools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ippools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipaddresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipaddresses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles Metal3Machine events
func (r *IPPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx = context.Background()
	metadataLog := r.Log.WithName(ipPoolControllerName).WithValues("metal3-ippool", req.NamespacedName)

	// Fetch the IPPool instance.
	ipamv1IPPool := &ipamv1.IPPool{}

	if err := r.Client.Get(ctx, req.NamespacedName, ipamv1IPPool); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	helper, err := patch.NewHelper(ipamv1IPPool, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch ipamv1IPPool exiting this function so we can persist any IPPool changes.
	defer func() {
		err := helper.Patch(ctx, ipamv1IPPool)
		if err != nil {
			metadataLog.Info("failed to Patch ipamv1IPPool")
		}
	}()

	cluster := &capi.Cluster{}
	if ipamv1IPPool.Spec.ClusterName != nil {
		key := client.ObjectKey{
			Name:      *ipamv1IPPool.Spec.ClusterName,
			Namespace: ipamv1IPPool.Namespace,
		}

		if ipamv1IPPool.ObjectMeta.Labels == nil {
			ipamv1IPPool.ObjectMeta.Labels = make(map[string]string)
		}
		ipamv1IPPool.ObjectMeta.Labels[capi.ClusterLabelName] = *ipamv1IPPool.Spec.ClusterName
		ipamv1IPPool.ObjectMeta.Labels[capi.ProviderLabelName] = "infrastructure-metal3"

		// Fetch the Cluster. Ignore an error if the deletion timestamp is set
		err = r.Client.Get(ctx, key, cluster)
		if ipamv1IPPool.ObjectMeta.DeletionTimestamp.IsZero() {
			if err != nil {
				metadataLog.Info("Error fetching cluster. It might not exist yet, Requeuing")
				return ctrl.Result{}, nil
			}
		}
	} else {
		cluster = nil
	}

	// Create a helper for managing the metadata object.
	ipPoolMgr, err := r.ManagerFactory.NewIPPoolManager(ipamv1IPPool, metadataLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the IP pool")
	}

	if ipamv1IPPool.Spec.ClusterName != nil && cluster != nil && cluster.Name != "" {
		metadataLog = metadataLog.WithValues("cluster", cluster.Name)
		if err := ipPoolMgr.SetClusterOwnerRef(cluster); err != nil {
			return ctrl.Result{}, err
		}

		// Return early if the Metadata or Cluster is paused.
		if annotations.IsPaused(cluster, ipamv1IPPool) {
			metadataLog.Info("reconciliation is paused for this object")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
		}
	}

	// Handle deleted metadata
	if !ipamv1IPPool.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, ipPoolMgr)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, ipPoolMgr)
}

func (r *IPPoolReconciler) reconcileNormal(ctx context.Context,
	ipPoolMgr ipam.IPPoolManagerInterface,
) (ctrl.Result, error) {
	// If the IPPool doesn't have finalizer, add it.
	ipPoolMgr.SetFinalizer()

	_, err := ipPoolMgr.UpdateAddresses(ctx)
	if err != nil {
		return checkRequeueError(err, "Failed to create the missing data")
	}

	return ctrl.Result{}, nil
}

func (r *IPPoolReconciler) reconcileDelete(ctx context.Context,
	ipPoolMgr ipam.IPPoolManagerInterface,
) (ctrl.Result, error) {

	allocationsNb, err := ipPoolMgr.UpdateAddresses(ctx)
	if err != nil {
		return checkRequeueError(err, "Failed to delete the old secrets")
	}

	if allocationsNb == 0 {
		// ippool is marked for deletion and ready to be deleted,
		// so remove the finalizer.
		ipPoolMgr.UnsetFinalizer()
	}

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller
func (r *IPPoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1.IPPool{}).
		Watches(
			&source.Kind{Type: &ipamv1.IPClaim{}},
			handler.EnqueueRequestsFromMapFunc(r.IPClaimToIPPool),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

// IPClaimToIPPool will return a reconcile request for a
// Metal3DataTemplate if the event is for a
// IPClaim and that IPClaim references a Metal3DataTemplate
func (r *IPPoolReconciler) IPClaimToIPPool(obj client.Object) []ctrl.Request {
	if m3ipc, ok := obj.(*ipamv1.IPClaim); ok {
		if m3ipc.Spec.Pool.Name != "" {
			namespace := m3ipc.Spec.Pool.Namespace
			if namespace == "" {
				namespace = m3ipc.Namespace
			}
			return []ctrl.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      m3ipc.Spec.Pool.Name,
						Namespace: namespace,
					},
				},
			}
		}
	}
	return []ctrl.Request{}
}

func checkRequeueError(err error, errMessage string) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}
	if requeueErr, ok := errors.Cause(err).(ipam.HasRequeueAfterError); ok {
		return ctrl.Result{Requeue: true, RequeueAfter: requeueErr.GetRequeueAfter()}, nil
	}
	return ctrl.Result{}, errors.Wrap(err, errMessage)
}
