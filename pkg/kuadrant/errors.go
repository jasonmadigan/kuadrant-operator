package kuadrant

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kuadrant/policy-machinery/machinery"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type PolicyError interface {
	error
	Reason() gatewayapiv1alpha2.PolicyConditionReason
}

var _ PolicyError = ErrTargetNotFound{}

type ErrTargetNotFound struct {
	Kind      string
	TargetRef gatewayapiv1alpha2.LocalPolicyTargetReference
	Err       error
}

func (e ErrTargetNotFound) Error() string {
	if apierrors.IsNotFound(e.Err) {
		return fmt.Sprintf("%s target %s was not found", e.Kind, e.TargetRef.Name)
	}

	return fmt.Sprintf("%s target %s was not found: %s", e.Kind, e.TargetRef.Name, e.Err.Error())
}

func (e ErrTargetNotFound) Reason() gatewayapiv1alpha2.PolicyConditionReason {
	return gatewayapiv1alpha2.PolicyReasonTargetNotFound
}

func NewErrTargetNotFound(kind string, targetRef gatewayapiv1alpha2.LocalPolicyTargetReference, err error) ErrTargetNotFound {
	return ErrTargetNotFound{
		Kind:      kind,
		TargetRef: targetRef,
		Err:       err,
	}
}

var _ PolicyError = ErrPolicyTargetNotFound{}

type ErrPolicyTargetNotFound struct {
	Kind      string
	TargetRef machinery.PolicyTargetReference
	Err       error
}

func (e ErrPolicyTargetNotFound) Error() string {
	if apierrors.IsNotFound(e.Err) {
		return fmt.Sprintf("%s target %s was not found", e.Kind, e.TargetRef.GetName())
	}

	return fmt.Sprintf("%s target %s was not found: %s", e.Kind, e.TargetRef.GetName(), e.Err.Error())
}

func (e ErrPolicyTargetNotFound) Reason() gatewayapiv1alpha2.PolicyConditionReason {
	return gatewayapiv1alpha2.PolicyReasonTargetNotFound
}

func NewErrPolicyTargetNotFound(kind string, targetRef machinery.PolicyTargetReference, err error) ErrPolicyTargetNotFound {
	return ErrPolicyTargetNotFound{
		Kind:      kind,
		TargetRef: targetRef,
		Err:       err,
	}
}

var _ PolicyError = ErrInvalid{}

type ErrInvalid struct {
	Kind string
	Err  error
}

func (e ErrInvalid) Error() string {
	return fmt.Sprintf("%s target is invalid: %s", e.Kind, e.Err.Error())
}

func (e ErrInvalid) Reason() gatewayapiv1alpha2.PolicyConditionReason {
	return gatewayapiv1alpha2.PolicyReasonInvalid
}

func NewErrInvalid(kind string, err error) ErrInvalid {
	return ErrInvalid{
		Kind: kind,
		Err:  err,
	}
}

var _ PolicyError = ErrConflict{}

type ErrConflict struct {
	Kind          string
	NameNamespace string
	Err           error
}

func (e ErrConflict) Error() string {
	return fmt.Sprintf("%s is conflicted by %s: %s", e.Kind, e.NameNamespace, e.Err.Error())
}

func (e ErrConflict) Reason() gatewayapiv1alpha2.PolicyConditionReason {
	return gatewayapiv1alpha2.PolicyReasonConflicted
}

func NewErrConflict(kind string, nameNamespace string, err error) ErrConflict {
	return ErrConflict{
		Kind:          kind,
		NameNamespace: nameNamespace,
		Err:           err,
	}
}

var _ PolicyError = ErrUnknown{}

type ErrUnknown struct {
	Kind string
	Err  error
}

func (e ErrUnknown) Error() string {
	return fmt.Sprintf("%s has encountered some issues: %s", e.Kind, e.Err.Error())
}

func (e ErrUnknown) Reason() gatewayapiv1alpha2.PolicyConditionReason {
	return PolicyReasonUnknown
}

func NewErrUnknown(kind string, err error) ErrUnknown {
	return ErrUnknown{
		Kind: kind,
		Err:  err,
	}
}

var _ PolicyError = ErrOverridden{}

type ErrNoRoutes struct {
	Kind string
}

func (e ErrNoRoutes) Error() string {
	return fmt.Sprintf("%s is not in the path to any existing routes", e.Kind)
}

func (e ErrNoRoutes) Reason() gatewayapiv1alpha2.PolicyConditionReason {
	return PolicyReasonUnknown
}

func NewErrNoRoutes(kind string) ErrNoRoutes {
	return ErrNoRoutes{
		Kind: kind,
	}
}

var _ PolicyError = ErrOverridden{}

type ErrOverridden struct {
	Kind               string
	OverridingPolicies []k8stypes.NamespacedName
}

func (e ErrOverridden) Error() string {
	if len(e.OverridingPolicies) == 0 {
		return fmt.Sprintf("%s is overridden", e.Kind)
	}
	return fmt.Sprintf("%s is overridden by %s", e.Kind, e.OverridingPolicies)
}

func (e ErrOverridden) Reason() gatewayapiv1alpha2.PolicyConditionReason {
	return PolicyReasonOverridden
}

func NewErrOverridden(kind string, overridingPolicies []k8stypes.NamespacedName) ErrOverridden {
	return ErrOverridden{
		Kind:               kind,
		OverridingPolicies: overridingPolicies,
	}
}

var _ PolicyError = ErrOutOfSync{}

type ErrOutOfSync struct {
	Kind       string
	Components []string
}

func (e ErrOutOfSync) Error() string {
	return fmt.Sprintf("%s waiting for the following components to sync: %s", e.Kind, e.Components)
}

func (e ErrOutOfSync) Reason() gatewayapiv1alpha2.PolicyConditionReason {
	return PolicyReasonUnknown
}

func NewErrOutOfSync(kind string, components []string) ErrOutOfSync {
	return ErrOutOfSync{
		Kind:       kind,
		Components: components,
	}
}

// IsTargetNotFound returns true if the specified error was created by NewErrTargetNotFound.
func IsTargetNotFound(err error) bool {
	return reasonForError(err) == gatewayapiv1alpha2.PolicyReasonTargetNotFound
}

func reasonForError(err error) gatewayapiv1alpha2.PolicyConditionReason {
	var policyErr PolicyError
	if errors.As(err, &policyErr) {
		return policyErr.Reason()
	}
	return ""
}

func NewErrDependencyNotInstalled(dependencyName ...string) ErrDependencyNotInstalled {
	return ErrDependencyNotInstalled{
		dependencyName: dependencyName,
	}
}

var _ PolicyError = ErrDependencyNotInstalled{}

type ErrDependencyNotInstalled struct {
	dependencyName []string
}

func (e ErrDependencyNotInstalled) Error() string {
	return fmt.Sprintf("[%s] is not installed, please restart Kuadrant Operator pod once dependency has been installed", strings.Join(e.dependencyName, ", "))
}

func (e ErrDependencyNotInstalled) Reason() gatewayapiv1alpha2.PolicyConditionReason {
	return PolicyReasonMissingDependency
}

func NewErrSystemResource(resourceName string) ErrSystemResource {
	return ErrSystemResource{
		resourceName: resourceName,
	}
}

var _ PolicyError = ErrSystemResource{}

type ErrSystemResource struct {
	resourceName string
}

func (e ErrSystemResource) Error() string {
	return fmt.Sprintf("%s is not installed, please create resource", e.resourceName)
}

func (e ErrSystemResource) Reason() gatewayapiv1alpha2.PolicyConditionReason {
	return PolicyReasonMissingResource
}
