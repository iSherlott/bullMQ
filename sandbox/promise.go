package sandbox

// Result is the resolved value of a Promise.
type Result[T any] struct {
	Value T
	Err   error
}

// Promise is a minimal Go equivalent of a JS Promise that resolves once.
// It is represented as a receive-only channel carrying a single Result.
type Promise[T any] <-chan Result[T]

// NewPromise runs fn in a goroutine and returns a Promise that resolves once.
func NewPromise[T any](fn func() (T, error)) Promise[T] {
	ch := make(chan Result[T], 1)
	go func() {
		var zero T
		v, err := fn()
		if err != nil {
			ch <- Result[T]{Value: zero, Err: err}
			return
		}
		ch <- Result[T]{Value: v, Err: nil}
	}()
	return ch
}
