package plan

// ErrorCode identifies a planning failure category.
type ErrorCode uint8

const (
	ErrorInvalidPlan ErrorCode = iota + 1
	ErrorInvalidExpression
	ErrorMissingColumn
	ErrorAmbiguousColumn
	ErrorTypeMismatch
	ErrorImpossibleCoercion
	ErrorUnsupportedOperation
)

// PlanError is a structured binding or physical-planning failure.
type PlanError struct {
	code    ErrorCode
	message string
}

// Code returns the stable error category.
func (e *PlanError) Code() ErrorCode {
	if e == nil {
		return 0
	}
	return e.code
}

// Error returns the planning error message.
func (e *PlanError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func makePlanError(code ErrorCode, message string) *PlanError {
	return &PlanError{code: code, message: message}
}
