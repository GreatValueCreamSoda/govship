package main

import "context"

func withContext[T any](ctx context.Context, ch <-chan T) <-chan T {
	out := make(chan T, 1)

	go func() {
		defer close(out)
		for val := range ch {
			select {
			case out <- val:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}
