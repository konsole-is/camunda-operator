package backupschedule

import (
	"fmt"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/generic"
)

// Builder is a configuration helper for creating and customizing the BackupSchedule Resource.
//
// It provides a fluent API for registering mutations, status handlers and declared
// data extractions. Build() validates the configuration and returns an initialized
// Resource ready for use in a reconciliation loop.
type Builder struct {
	base *generic.WorkloadBuilder[*apiv1.BackupSchedule, *Mutator]
}

// NewBuilder initializes a new Builder with the provided BackupSchedule object.
//
// The BackupSchedule object serves as the desired base state. During reconciliation the
// framework makes the cluster's state match this base state, modified by any
// registered mutations.
//
// The provided BackupSchedule must have a Name set and a Namespace set, which is
// validated during the Build() call.
func NewBuilder(obj *apiv1.BackupSchedule) *Builder {
	identityFunc := func(o *apiv1.BackupSchedule) string {
		return fmt.Sprintf("core.camunda.io/v1/BackupSchedule/%s/%s", o.Namespace, o.Name)
	}

	base := generic.NewWorkloadBuilder[*apiv1.BackupSchedule, *Mutator](
		obj,
		identityFunc,
		NewMutator,
	)

	base.
		WithCustomConvergeStatus(DefaultConvergingStatusHandler).
		WithCustomGraceStatus(DefaultGraceStatusHandler).
		WithCustomSuspendStatus(DefaultSuspensionStatusHandler).
		WithCustomSuspendMutation(DefaultSuspendMutationHandler).
		WithCustomSuspendDeletionDecision(DefaultDeleteOnSuspendHandler)

	return &Builder{
		base: base,
	}
}

// WithMutation registers one or more feature-based mutations for the BackupSchedule.
//
// Mutations are applied sequentially during the Mutate() phase of reconciliation.
// A mutation with a nil Feature is applied unconditionally; one with a non-nil
// Feature is applied only when that feature is enabled.
func (b *Builder) WithMutation(ms ...Mutation) *Builder {
	for _, m := range ms {
		b.base.WithMutation(feature.Mutation[*Mutator](m))
	}
	return b
}

// WithCustomConvergeStatus overrides the default logic for determining whether the
// BackupSchedule has reached its converged state.
//
// The default behavior uses DefaultConvergingStatusHandler, which reports Healthy
// unconditionally. This handler is required by the generic layer, so it is registered
// in NewBuilder and can only be replaced, never cleared.
func (b *Builder) WithCustomConvergeStatus(
	handler func(concepts.ConvergingOperation, *apiv1.BackupSchedule) (concepts.AliveStatusWithReason, error),
) *Builder {
	b.base.WithCustomConvergeStatus(handler)
	return b
}

// WithCustomGraceStatus overrides how the BackupSchedule reports its health once the
// component's grace period has expired.
//
// The default behavior uses DefaultGraceStatusHandler.
func (b *Builder) WithCustomGraceStatus(
	handler func(*apiv1.BackupSchedule) (concepts.GraceStatusWithReason, error),
) *Builder {
	b.base.WithCustomGraceStatus(handler)
	return b
}

// WithCustomSuspendStatus overrides how the progress of suspension is reported.
//
// The default behavior uses DefaultSuspensionStatusHandler.
func (b *Builder) WithCustomSuspendStatus(
	handler func(*apiv1.BackupSchedule) (concepts.SuspensionStatusWithReason, error),
) *Builder {
	b.base.WithCustomSuspendStatus(handler)
	return b
}

// WithCustomSuspendMutation defines how the BackupSchedule is modified when the component
// is suspended.
//
// The default behavior uses DefaultSuspendMutationHandler.
func (b *Builder) WithCustomSuspendMutation(handler func(*Mutator) error) *Builder {
	b.base.WithCustomSuspendMutation(handler)
	return b
}

// WithCustomSuspendDeletionDecision overrides the decision of whether to delete the
// BackupSchedule when the component is suspended.
//
// The default behavior uses DefaultDeleteOnSuspendHandler.
func (b *Builder) WithCustomSuspendDeletionDecision(handler func(*apiv1.BackupSchedule) bool) *Builder {
	b.base.WithCustomSuspendDeletionDecision(handler)
	return b
}

// WithGuard registers a guard precondition that is evaluated before the BackupSchedule is
// applied during reconciliation. If the guard returns Blocked, the BackupSchedule and all
// resources registered after it are skipped until the guard clears.
// Passing nil clears any previously registered guard.
func (b *Builder) WithGuard(
	guard func(apiv1.BackupSchedule) (concepts.GuardStatusWithReason, error),
) *Builder {
	b.base.WithGuard(generic.WrapGuard(guard))
	return b
}

// WithDataGuard declares that the BackupSchedule reads the given data cells and must not
// be applied until every one of them is set. The framework generates the guard and
// its reason (waiting for data "<name>"), and component Build validates that a
// producer for each cell is registered earlier. Data guards are evaluated before any
// custom guard registered with WithGuard.
func (b *Builder) WithDataGuard(cells ...concepts.DataCell) *Builder {
	b.base.WithDataGuard(cells...)
	return b
}

// WithOptionalData declares that the BackupSchedule reads the given data cells without
// gating on them. Component Build still validates that a producer is registered
// earlier, and the dependency stays visible to introspection. Consumers in this mode
// use Get and skip quietly when a cell is absent.
func (b *Builder) WithOptionalData(cells ...concepts.DataCell) *Builder {
	b.base.WithOptionalData(cells...)
	return b
}

// WithMetricsIdentifier sets the BackupSchedule's identifier for resource-level metrics,
// used as the value of the `resource` label on ocf_resource_apply_total and
// ocf_resource_apply_errors_total.
//
// It is a Prometheus label value, not a Kubernetes name: it must be low-cardinality
// and stable across reconciles, never derived from a per-owner value such as the
// owning custom resource's name. When unset, the resource is labelled
// `backupschedule`. Build rejects a blank identifier.
func (b *Builder) WithMetricsIdentifier(identifier string) *Builder {
	b.base.WithMetricsIdentifier(identifier)
	return b
}

// Build validates the configuration and returns the initialized Resource.
//
// It returns an error if:
//   - No BackupSchedule object was provided.
//   - The BackupSchedule is missing a Name.
//   - The BackupSchedule is missing a Namespace.
//   - Two registered mutations share a name.
func (b *Builder) Build() (*Resource, error) {
	genericRes, err := b.base.Build()
	if err != nil {
		return nil, err
	}
	return &Resource{base: genericRes}, nil
}

// ExtractInto declares that this BackupSchedule produces the value of cell. fn computes
// the value from a copy of the reconciled BackupSchedule; the framework stores it in the
// cell and marks it present, immediately after the BackupSchedule is applied or fetched.
// Extracting several values means several ExtractInto calls, one per cell. This is a
// package-level function because Go methods cannot introduce the extra type
// parameter V.
func ExtractInto[V any](b *Builder, cell *concepts.Data[V], fn func(apiv1.BackupSchedule) (V, error)) {
	generic.ExtractInto(&b.base.BaseBuilder, cell, generic.WrapExtraction(fn))
}
