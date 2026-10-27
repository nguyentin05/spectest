package generate

import (
	"context"
	"errors"
	"sync"
)

// runBounded runs fn over items with at most `concurrency` calls in flight.
// Results are stored by index, so output order never depends on completion
// order. The first failure cancels the remaining work; cancellation noise from
// peers is dropped when the errors are joined. concurrency <= 1 is sequential
// and fail-fast.
func runBounded[T, R any](
	ctx context.Context,
	items []T,
	concurrency int,
	fn func(ctx context.Context, index int, item T) (R, error),
) ([]R, error) {
	out := make([]R, len(items))
	if len(items) == 0 {
		return out, nil
	}
	n := max(1, min(concurrency, len(items)))

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int)
	errs := make([]error, len(items))
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			for i := range jobs {
				if runCtx.Err() != nil {
					continue
				}
				r, err := fn(runCtx, i, items[i])
				if err != nil {
					errs[i] = err
					cancel()
					continue
				}
				out[i] = r
			}
		})
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var joined []error
	for _, e := range errs {
		if e != nil && !errors.Is(e, context.Canceled) {
			joined = append(joined, e)
		}
	}
	if len(joined) > 0 {
		return nil, errors.Join(joined...)
	}
	return out, nil
}

// runAllBounded runs fn over items with at most `concurrency` calls in flight.
// Unlike runBounded, an error in one item does NOT cancel the context for the
// remaining items; all items are processed. Output results and errors are
// returned by index. If the outer context is canceled, any unstarted items
// receive ctx.Err().
func runAllBounded[T, R any](
	ctx context.Context,
	items []T,
	concurrency int,
	fn func(ctx context.Context, index int, item T) (R, error),
) ([]R, []error) {
	out := make([]R, len(items))
	errs := make([]error, len(items))
	if len(items) == 0 {
		return out, errs
	}
	n := max(1, min(concurrency, len(items)))

	jobs := make(chan int, len(items))
	for i := range items {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			for i := range jobs {
				if ctx.Err() != nil {
					errs[i] = ctx.Err()
					continue
				}
				r, err := fn(ctx, i, items[i])
				if err != nil {
					errs[i] = err
				} else {
					out[i] = r
				}
			}
		})
	}
	wg.Wait()
	return out, errs
}
