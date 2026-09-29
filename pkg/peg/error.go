// Package peg provides a lazy Go expression API over the peGosus query engine.
package peg

import (
	"errors"
	"fmt"

	"github.com/rhawrami/peGosus/pkg/io/csv"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/plan"
)

// ErrorCode classifies public planning and execution failures.
type ErrorCode uint8

const (
	ErrorInvalidQuery ErrorCode = iota + 1
	ErrorInvalidExpression
	ErrorMissingColumn
	ErrorAmbiguousColumn
	ErrorTypeMismatch
	ErrorImpossibleCoercion
	ErrorUnsupported
	ErrorSource
	ErrorResourceExhausted
	ErrorCancelled
	ErrorExecution
)

// Error is a structured public query failure. Unwrap exposes the underlying
// planner, reader, filesystem, or cancellation error.
type Error struct {
	code  ErrorCode
	cause error
}

// Code returns the failure category.
func (e *Error) Code() ErrorCode {
	if e == nil {
		return 0
	}
	return e.code
}

// Error returns the underlying failure description.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprint(e.cause)
}

// Unwrap exposes the underlying failure.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func wrapError(err error) error {
	if err == nil {
		return nil
	}
	var public *Error
	if errors.As(err, &public) {
		return err
	}
	var binding *plan.PlanError
	if errors.As(err, &binding) {
		code := ErrorInvalidQuery
		switch binding.Code() {
		case plan.ErrorInvalidExpression:
			code = ErrorInvalidExpression
		case plan.ErrorMissingColumn:
			code = ErrorMissingColumn
		case plan.ErrorAmbiguousColumn:
			code = ErrorAmbiguousColumn
		case plan.ErrorTypeMismatch:
			code = ErrorTypeMismatch
		case plan.ErrorImpossibleCoercion:
			code = ErrorImpossibleCoercion
		case plan.ErrorUnsupportedOperation:
			code = ErrorUnsupported
		}
		return &Error{code: code, cause: err}
	}
	var source *parquet.ParquetError
	if errors.As(err, &source) {
		code := ErrorSource
		switch source.Code() {
		case parquet.ParquetUnsupported:
			code = ErrorUnsupported
		case parquet.ParquetResourceExhausted:
			code = ErrorResourceExhausted
		case parquet.ParquetCancelled:
			code = ErrorCancelled
		}
		return &Error{code: code, cause: err}
	}
	var csvSource *csv.CSVError
	if errors.As(err, &csvSource) {
		code := ErrorSource
		switch csvSource.Code() {
		case csv.CSVResourceExhausted:
			code = ErrorResourceExhausted
		case csv.CSVCancelled:
			code = ErrorCancelled
		}
		return &Error{code: code, cause: err}
	}
	return &Error{code: ErrorSource, cause: err}
}
