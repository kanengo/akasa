package codegen

import (
	"errors"
)

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

type resultUnwrapError struct {
	err error
}

func WrapResultError(err error) error {
	return resultUnwrapError{err: err}
}

func (e resultUnwrapError) Error() string {
	return e.err.Error()
}

func (e resultUnwrapError) Unwrap() error {
	return e.err
}

func CatchResultUnwrapPanic(r any) error {
	if r == nil {
		return nil
	}

	err, ok := r.(error)
	if !ok {
		panic(r)
	}

	var uErr resultUnwrapError
	if errors.As(err, &uErr) {
		return uErr.Unwrap()
	}

	panic(err)
}
