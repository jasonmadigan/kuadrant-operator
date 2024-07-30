package controllers

import (
	"context"

	"github.com/kuadrant/kuadrant-operator/api/v1beta2"
	"github.com/kuadrant/policy-machinery/machinery"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// EffectivePolicyReconciler reconciles an EffectivePolicy object
type EffectivePolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kuadrant.io,resources=effectivepolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kuadrant.io,resources=effectivepolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kuadrant.io,resources=effectivepolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.k8s.io,resources=httproutes,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=gateways,verbs=get;list;watch

// Reconcile is part of the main Kubernetes reconciliation loop
func (r *EffectivePolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	const policyName = "cluster-effectivepolicy"

	log.Info("Reconciling EffectivePolicy", "request", req)

	// Fetch or create the single EffectivePolicy instance
	var ep v1beta2.EffectivePolicy
	err := r.Get(ctx, client.ObjectKey{Name: policyName}, &ep)
	if err != nil {
		if errors.IsNotFound(err) {
			// If not found, create a new one with a fixed name
			newEP := &v1beta2.EffectivePolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: policyName,
				},
				Spec: v1beta2.EffectivePolicySpec{
					Dotfile: "initializing dotfile...",
				},
			}
			if err := r.Create(ctx, newEP); err != nil {
				log.Error(err, "Failed to create EffectivePolicy")
				return ctrl.Result{}, err
			}
			log.Info("Created new EffectivePolicy", "name", policyName)
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get EffectivePolicy")
		return ctrl.Result{}, err
	}

	// Create or update topology and update the dotfile field
	topology := r.buildTopology(ctx)
	if topology == nil {
		log.Error(nil, "Failed to build topology")
		return ctrl.Result{}, nil
	}
	ep.Spec.Dotfile = topology.ToDot().String()

	if err := r.Update(ctx, &ep); err != nil {
		log.Error(err, "Failed to update EffectivePolicy")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *EffectivePolicyReconciler) buildTopology(ctx context.Context) *machinery.Topology {
	// Fetch HTTPRoute resources
	var httpRoutes gatewayapiv1.HTTPRouteList
	if err := r.List(ctx, &httpRoutes); err != nil {
		// handle
	}

	var gateways gatewayapiv1.GatewayList
	if err := r.List(ctx, &gateways); err != nil {
		// handle
	}

	// Create the topology
	topology := machinery.NewGatewayAPITopology(
		machinery.WithHTTPRoutes(lo.Map(httpRoutes.Items, func(hr gatewayapiv1.HTTPRoute, _ int) *gatewayapiv1.HTTPRoute {
			return &hr
		})...),
		machinery.WithGateways(lo.Map(gateways.Items, func(hr gatewayapiv1.Gateway, _ int) *gatewayapiv1.Gateway {
			return &hr
		})...),
		// Add our other policies
	)

	return topology
}

// SetupWithManager sets up the controller with the Manager.
func (r *EffectivePolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.EffectivePolicy{}).
		// Watch for changes to Gateway and HTTPRoute resources
		Watches(&gatewayapiv1.HTTPRoute{}, &handler.EnqueueRequestForObject{}).
		Watches(&gatewayapiv1.Gateway{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
