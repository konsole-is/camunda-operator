package barmanobjectstore

import (
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/generic"
)

// Builder is a configuration helper for creating and customizing the ObjectStore Resource.
//
// It provides a fluent API for registering mutations and declared data extractions.
// Build() validates the configuration and returns an initialized Resource ready for
// use in a reconciliation loop.
type Builder struct {
	base *generic.StaticBuilder[*ObjectStore, *Mutator]
}

// NewBuilder initializes a new Builder with the provided ObjectStore object.
//
// The ObjectStore object serves as the desired base state. During reconciliation the
// framework makes the cluster's state match this base state, modified by any
// registered mutations.
//
// The provided ObjectStore must have a Name set and a Namespace set, which is
// validated during the Build() call.
func NewBuilder(obj *ObjectStore) *Builder {
	identityFunc := func(o *ObjectStore) string {
		return fmt.Sprintf("barmancloud.cnpg.io/v1/ObjectStore/%s/%s", o.Namespace, o.Name)
	}

	base := generic.NewStaticBuilder[*ObjectStore, *Mutator](
		obj,
		identityFunc,
		NewMutator,
	)

	return &Builder{
		base: base,
	}
}

// WithMutation registers one or more feature-based mutations for the ObjectStore.
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

// WithGuard registers a guard precondition that is evaluated before the ObjectStore is
// applied during reconciliation. If the guard returns Blocked, the ObjectStore and all
// resources registered after it are skipped until the guard clears.
// Passing nil clears any previously registered guard.
func (b *Builder) WithGuard(
	guard func(ObjectStore) (concepts.GuardStatusWithReason, error),
) *Builder {
	b.base.WithGuard(generic.WrapGuard(guard))
	return b
}

// WithDataGuard declares that the ObjectStore reads the given data cells and must not
// be applied until every one of them is set. The framework generates the guard and
// its reason (waiting for data "<name>"), and component Build validates that a
// producer for each cell is registered earlier. Data guards are evaluated before any
// custom guard registered with WithGuard.
func (b *Builder) WithDataGuard(cells ...concepts.DataCell) *Builder {
	b.base.WithDataGuard(cells...)
	return b
}

// WithOptionalData declares that the ObjectStore reads the given data cells without
// gating on them. Component Build still validates that a producer is registered
// earlier, and the dependency stays visible to introspection. Consumers in this mode
// use Get and skip quietly when a cell is absent.
func (b *Builder) WithOptionalData(cells ...concepts.DataCell) *Builder {
	b.base.WithOptionalData(cells...)
	return b
}

// Build validates the configuration and returns the initialized Resource.
//
// It returns an error if:
//   - No ObjectStore object was provided.
//   - The ObjectStore is missing a Name.
//   - The ObjectStore is missing a Namespace.
//   - Two registered mutations share a name.
func (b *Builder) Build() (*Resource, error) {
	genericRes, err := b.base.Build()
	if err != nil {
		return nil, err
	}
	return &Resource{base: genericRes}, nil
}

// ExtractInto declares that this ObjectStore produces the value of cell. fn computes
// the value from a copy of the reconciled ObjectStore; the framework stores it in the
// cell and marks it present, immediately after the ObjectStore is applied or fetched.
// Extracting several values means several ExtractInto calls, one per cell. This is a
// package-level function because Go methods cannot introduce the extra type
// parameter V.
func ExtractInto[V any](b *Builder, cell *concepts.Data[V], fn func(ObjectStore) (V, error)) {
	generic.ExtractInto(&b.base.BaseBuilder, cell, generic.WrapExtraction(fn))
}
