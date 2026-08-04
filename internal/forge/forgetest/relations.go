package forgetest

import (
	"context"

	"github.com/tnikic/anvil/internal/forge"
)

// FakeRelationService is a state-based fake of forge.RelationService.
// Pre-populate BlockedByItems, BlockingItems, ChildrenItems, and ParentItem
// to define responses. Fn overrides take precedence when set.
type FakeRelationService struct {
	BlockedByItems []forge.IssueDependency
	BlockingItems  []forge.IssueDependency
	ChildrenItems  []forge.IssueDependency
	ParentItem     *forge.IssueDependency

	BlockedByFn func(ctx context.Context, number int) ([]forge.IssueDependency, error)
	BlockingFn  func(ctx context.Context, number int) ([]forge.IssueDependency, error)
	ChildrenFn  func(ctx context.Context, number int) ([]forge.IssueDependency, error)
	ParentFn    func(ctx context.Context, number int) (*forge.IssueDependency, error)

	AddBlocksFn      func(ctx context.Context, number int, target int) error
	RemoveBlocksFn   func(ctx context.Context, number int, target int) error
	AddParentOfFn    func(ctx context.Context, number int, child int) error
	RemoveParentOfFn func(ctx context.Context, number int, child int) error

	// Last* captures the most recent call argument for assertion.
	LastBlockedByNumber int
	LastBlockingNumber  int
	LastChildrenNumber  int
	LastParentNumber    int

	LastAddBlocksNumber      int
	LastAddBlocksTarget      int
	LastRemoveBlocksNumber   int
	LastRemoveBlocksTarget   int
	LastAddParentOfNumber    int
	LastAddParentOfChild     int
	LastRemoveParentOfNumber int
	LastRemoveParentOfChild  int

	// Blocks and ParentOf are state trackers for idempotency simulation.
	Blocks   map[int][]int // number -> targets it blocks
	ParentOf map[int]int   // child -> parent
}

var _ forge.RelationService = (*FakeRelationService)(nil)

func (s *FakeRelationService) BlockedBy(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	s.LastBlockedByNumber = number
	if s.BlockedByFn != nil {
		return s.BlockedByFn(ctx, number)
	}
	if s.BlockedByItems == nil {
		return []forge.IssueDependency{}, nil
	}
	return s.BlockedByItems, nil
}

func (s *FakeRelationService) Blocking(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	s.LastBlockingNumber = number
	if s.BlockingFn != nil {
		return s.BlockingFn(ctx, number)
	}
	if s.BlockingItems == nil {
		return []forge.IssueDependency{}, nil
	}
	return s.BlockingItems, nil
}

func (s *FakeRelationService) Children(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	s.LastChildrenNumber = number
	if s.ChildrenFn != nil {
		return s.ChildrenFn(ctx, number)
	}
	if s.ChildrenItems == nil {
		return []forge.IssueDependency{}, nil
	}
	return s.ChildrenItems, nil
}

func (s *FakeRelationService) Parent(ctx context.Context, number int) (*forge.IssueDependency, error) {
	s.LastParentNumber = number
	if s.ParentFn != nil {
		return s.ParentFn(ctx, number)
	}
	return s.ParentItem, nil
}

func (s *FakeRelationService) AddBlocks(ctx context.Context, number int, target int) error {
	s.LastAddBlocksNumber = number
	s.LastAddBlocksTarget = target
	if s.AddBlocksFn != nil {
		return s.AddBlocksFn(ctx, number, target)
	}
	// State-based: track in Blocks map.
	if s.Blocks == nil {
		s.Blocks = make(map[int][]int)
	}
	for _, t := range s.Blocks[number] {
		if t == target {
			return nil // already present, idempotent
		}
	}
	s.Blocks[number] = append(s.Blocks[number], target)
	// Also update BlockedByItems for read consistency.
	s.BlockedByItems = append(s.BlockedByItems, forge.IssueDependency{
		Number:    number,
		Title:     "fake",
		State:     forge.StateOpen,
		Direction: forge.DirBlockedBy,
	})
	return nil
}

func (s *FakeRelationService) RemoveBlocks(ctx context.Context, number int, target int) error {
	s.LastRemoveBlocksNumber = number
	s.LastRemoveBlocksTarget = target
	if s.RemoveBlocksFn != nil {
		return s.RemoveBlocksFn(ctx, number, target)
	}
	if s.Blocks == nil {
		return nil
	}
	targets := s.Blocks[number]
	for i, t := range targets {
		if t == target {
			s.Blocks[number] = append(targets[:i], targets[i+1:]...)
			// Remove from BlockedByItems for read consistency.
			for j, d := range s.BlockedByItems {
				if d.Number == number {
					s.BlockedByItems = append(s.BlockedByItems[:j], s.BlockedByItems[j+1:]...)
					break
				}
			}
			return nil
		}
	}
	return nil // not found, idempotent
}

func (s *FakeRelationService) AddParentOf(ctx context.Context, number int, child int) error {
	s.LastAddParentOfNumber = number
	s.LastAddParentOfChild = child
	if s.AddParentOfFn != nil {
		return s.AddParentOfFn(ctx, number, child)
	}
	if s.ParentOf == nil {
		s.ParentOf = make(map[int]int)
	}
	if existing, ok := s.ParentOf[child]; ok && existing == number {
		return nil // already parent, idempotent
	}
	s.ParentOf[child] = number
	s.ParentItem = &forge.IssueDependency{
		Number:    number,
		Title:     "fake",
		State:     forge.StateOpen,
		Direction: forge.DirParent,
	}
	return nil
}

func (s *FakeRelationService) RemoveParentOf(ctx context.Context, number int, child int) error {
	s.LastRemoveParentOfNumber = number
	s.LastRemoveParentOfChild = child
	if s.RemoveParentOfFn != nil {
		return s.RemoveParentOfFn(ctx, number, child)
	}
	if s.ParentOf == nil {
		return nil
	}
	if existing, ok := s.ParentOf[child]; ok && existing == number {
		delete(s.ParentOf, child)
		s.ParentItem = nil
	}
	return nil // not found or wrong parent, idempotent
}
