package forge

import "context"

// RelationMutationFunc is the raw API mutation function provided by each
// forge adapter. It receives the context and issue numbers and performs
// the actual forge API call (without idempotency checks).
type RelationMutationFunc func(ctx context.Context, number, target int) error

// RelationReader is the read-side of RelationService: the four query
// methods that the RelationGuard uses for idempotency checks.
type RelationReader interface {
	BlockedBy(ctx context.Context, number int) ([]IssueDependency, error)
	Blocking(ctx context.Context, number int) ([]IssueDependency, error)
	Children(ctx context.Context, number int) ([]IssueDependency, error)
	Parent(ctx context.Context, number int) (*IssueDependency, error)
}

// RelationGuard wraps a RelationReader and adds idempotency guards to
// relation mutations. Each adapter provides the forge-specific mutation
// functions; the guard handles the read-check-mutate pattern.
//
// The read methods (BlockedBy, Blocking, Children, Parent) are delegated
// directly to the inner RelationReader.
type RelationGuard struct {
	inner            RelationReader
	addBlocksFn      RelationMutationFunc
	removeBlocksFn   RelationMutationFunc
	addParentOfFn    RelationMutationFunc
	removeParentOfFn RelationMutationFunc
}

// NewRelationGuard creates a RelationGuard that wraps inner and uses the
// provided mutation functions for the actual forge API calls. The guard
// handles idempotency checks (e.g., AddBlocks is a no-op if the
// relationship already exists).
func NewRelationGuard(inner RelationReader, addBlocks, removeBlocks, addParentOf, removeParentOf RelationMutationFunc) *RelationGuard {
	return &RelationGuard{
		inner:            inner,
		addBlocksFn:      addBlocks,
		removeBlocksFn:   removeBlocks,
		addParentOfFn:    addParentOf,
		removeParentOfFn: removeParentOf,
	}
}

// BlockedBy delegates to the inner RelationService.
func (g *RelationGuard) BlockedBy(ctx context.Context, number int) ([]IssueDependency, error) {
	return g.inner.BlockedBy(ctx, number)
}

// Blocking delegates to the inner RelationService.
func (g *RelationGuard) Blocking(ctx context.Context, number int) ([]IssueDependency, error) {
	return g.inner.Blocking(ctx, number)
}

// Children delegates to the inner RelationService.
func (g *RelationGuard) Children(ctx context.Context, number int) ([]IssueDependency, error) {
	return g.inner.Children(ctx, number)
}

// Parent delegates to the inner RelationService.
func (g *RelationGuard) Parent(ctx context.Context, number int) (*IssueDependency, error) {
	return g.inner.Parent(ctx, number)
}

// AddBlocks makes `number` block `target`. Idempotent: no-op if the
// relationship already exists.
func (g *RelationGuard) AddBlocks(ctx context.Context, number, target int) error {
	existing, err := g.inner.BlockedBy(ctx, target)
	if err != nil {
		return err
	}
	for _, d := range existing {
		if d.Number == number {
			return nil // already blocks
		}
	}
	return g.addBlocksFn(ctx, number, target)
}

// RemoveBlocks removes the "number blocks target" relationship.
// Idempotent: no-op if the relationship doesn't exist.
func (g *RelationGuard) RemoveBlocks(ctx context.Context, number, target int) error {
	existing, err := g.inner.BlockedBy(ctx, target)
	if err != nil {
		return err
	}
	for _, d := range existing {
		if d.Number == number {
			return g.removeBlocksFn(ctx, number, target)
		}
	}
	return nil // already not blocking
}

// AddParentOf makes `number` the parent of `child`. Idempotent: no-op if
// `number` is already the parent.
func (g *RelationGuard) AddParentOf(ctx context.Context, number, child int) error {
	existing, err := g.inner.Parent(ctx, child)
	if err != nil {
		return err
	}
	if existing != nil && existing.Number == number {
		return nil // already parent
	}
	return g.addParentOfFn(ctx, number, child)
}

// RemoveParentOf removes the parent/child relationship between `number`
// and `child`. Idempotent: no-op if the relationship doesn't exist.
func (g *RelationGuard) RemoveParentOf(ctx context.Context, number, child int) error {
	existing, err := g.inner.Parent(ctx, child)
	if err != nil {
		return err
	}
	if existing == nil || existing.Number != number {
		return nil // already not parent
	}
	return g.removeParentOfFn(ctx, number, child)
}
