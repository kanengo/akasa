package codegen

import "errors"

func CatchPanics(r any) error {
	if r == nil {
		return nil
	}

	err, ok := r.(error)
	if !ok {
		panic(r)
	}

	if errors.As(err, &serializerError{}) || errors.As(err, &deserializerError{}) {
		return err
	}

	panic(r)
}
