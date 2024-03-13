package akasa

type Ref[T any] struct {
	val T
}

func (ref *Ref[T]) Get() T {
	return ref.val
}

type Result[T any] struct {
	val T
	err error
}

func (r *Result[T]) Unwrap() T {
	if r.err != nil {
		panic(r.err)
	}
	return r.val
}

func (r *Result[T]) Error() error {
	return r.err
}

type Components[T any] struct {
}
