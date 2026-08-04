package forge_test

import (
	"errors"
	"testing"

	"github.com/tnikic/anvil/internal/forge"
)

// ---- StringVal ----

func TestStringValNonNil(t *testing.T) {
	s := "hello"
	if got := forge.StringVal(&s); got != "hello" {
		t.Errorf("StringVal(%q) = %q, want %q", "hello", got, "hello")
	}
}

func TestStringValNil(t *testing.T) {
	if got := forge.StringVal(nil); got != "" {
		t.Errorf("StringVal(nil) = %q, want empty string", got)
	}
}

// ---- Paginate ----

func TestPaginateSinglePage(t *testing.T) {
	items, err := forge.Paginate(0, func(page int) (forge.Page[string], error) {
		if page != 1 {
			t.Errorf("expected page 1, got %d", page)
		}
		return forge.Page[string]{Items: []string{"a", "b"}, NextPage: 0}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0] != "a" || items[1] != "b" {
		t.Errorf("items = %v, want [a b]", items)
	}
}

func TestPaginateMultiPage(t *testing.T) {
	callCount := 0
	items, err := forge.Paginate(0, func(page int) (forge.Page[int], error) {
		callCount++
		if page == 1 {
			return forge.Page[int]{Items: []int{1, 2}, NextPage: 2}, nil
		}
		if page == 2 {
			return forge.Page[int]{Items: []int{3, 4}, NextPage: 0}, nil
		}
		t.Errorf("unexpected page: %d", page)
		return forge.Page[int]{}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 fetch calls, got %d", callCount)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}
}

func TestPaginateLimitReached(t *testing.T) {
	callCount := 0
	items, err := forge.Paginate(3, func(page int) (forge.Page[int], error) {
		callCount++
		if page == 1 {
			return forge.Page[int]{Items: []int{1, 2}, NextPage: 2}, nil
		}
		if page == 2 {
			return forge.Page[int]{Items: []int{3, 4}, NextPage: 3}, nil
		}
		t.Errorf("should not call page %d after limit reached", page)
		return forge.Page[int]{}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 fetch calls, got %d", callCount)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 items (page is fetched fully even if limit <= items on page), got %d", len(items))
	}
}

func TestPaginateEmptyResult(t *testing.T) {
	items, err := forge.Paginate(0, func(page int) (forge.Page[string], error) {
		return forge.Page[string]{Items: []string{}, NextPage: 0}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestPaginateError(t *testing.T) {
	expectedErr := errors.New("fetch error")
	_, err := forge.Paginate(0, func(page int) (forge.Page[string], error) {
		return forge.Page[string]{}, expectedErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("got error %v, want %v", err, expectedErr)
	}
}

func TestPaginateErrorOnSecondPage(t *testing.T) {
	expectedErr := errors.New("second page error")
	callCount := 0
	_, err := forge.Paginate(0, func(page int) (forge.Page[int], error) {
		callCount++
		if page == 1 {
			return forge.Page[int]{Items: []int{1}, NextPage: 2}, nil
		}
		return forge.Page[int]{}, expectedErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("got error %v, want %v", err, expectedErr)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}
