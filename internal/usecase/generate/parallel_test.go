package generate

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestRunAllBounded_Success(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	out, errs := runAllBounded(context.Background(), items, 3, func(ctx context.Context, idx int, item int) (int, error) {
		return item * 10, nil
	})

	for i, err := range errs {
		if err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}
	}
	expected := []int{10, 20, 30, 40, 50}
	for i, v := range out {
		if v != expected[i] {
			t.Errorf("at %d: got %d, want %d", i, v, expected[i])
		}
	}
}

func TestRunAllBounded_ResilientToErrors(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	sentinel := errors.New("boom")
	out, errs := runAllBounded(context.Background(), items, 2, func(ctx context.Context, idx int, item int) (int, error) {
		if item == 3 {
			return 0, sentinel
		}
		return item * 2, nil
	})

	if len(errs) != 5 {
		t.Fatalf("expected 5 errs, got %d", len(errs))
	}
	if !errors.Is(errs[2], sentinel) {
		t.Errorf("expected sentinel error at index 2, got %v", errs[2])
	}
	// Other items should have succeeded despite index 2 failing
	for i, err := range errs {
		if i != 2 && err != nil {
			t.Errorf("expected nil error at %d, got %v", i, err)
		}
	}
	expected := []int{2, 4, 0, 8, 10}
	for i, v := range out {
		if v != expected[i] {
			t.Errorf("at %d: got %d, want %d", i, v, expected[i])
		}
	}
}

func TestRunAllBounded_Empty(t *testing.T) {
	out, errs := runAllBounded(context.Background(), []int{}, 4, func(ctx context.Context, idx int, item int) (int, error) {
		return item, nil
	})
	if len(out) != 0 || len(errs) != 0 {
		t.Errorf("expected empty slices, got len(out)=%d, len(errs)=%d", len(out), len(errs))
	}
}

func TestRunAllBounded_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	items := []int{1, 2, 3, 4, 5}
	var started atomic.Int32

	out, errs := runAllBounded(ctx, items, 1, func(c context.Context, idx int, item int) (int, error) {
		s := started.Add(1)
		if s == 1 {
			cancel()
			return item, nil
		}
		return item, nil
	})

	if len(out) != 5 {
		t.Fatalf("expected 5 items in out, got %d", len(out))
	}
	// At least one of the later items should have context.Canceled error
	hasCanceled := false
	for _, err := range errs {
		if errors.Is(err, context.Canceled) {
			hasCanceled = true
			break
		}
	}
	if !hasCanceled {
		t.Errorf("expected at least one context.Canceled error in errs")
	}
}
