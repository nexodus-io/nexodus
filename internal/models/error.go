package models

import "fmt"

// BaseError is the base type for API errors
type BaseError struct {
	Error string `json:"error" example:"something bad"`
}

// NewApiError returns a new response body for a general error
func NewApiError(err error) BaseError {
	return BaseError{
		Error: err.Error(),
	}
}

func NewBaseError(error string) BaseError {
	return BaseError{
		Error: error,
	}
}

// ValidationError is returned in the body of an HTTP 400
type ValidationError struct {
	BaseError
	Field  string `json:"field,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type InternalServerError struct {
	BaseError
	TraceId string `json:"trace_id,omitempty"`
}

func NewBadPayloadError(err error) ValidationError {
	return ValidationError{
		BaseError: BaseError{
			Error: fmt.Sprintf("request json is invalid: %s", err),
		},
	}
}

func NewBadPathParameterError(param string) ValidationError {
	return ValidationError{
		Field: param,
		BaseError: BaseError{
			Error: "path parameter invalid",
		},
	}
}

func NewBadPathParameterErrorAndReason(param string, reason string) ValidationError {
	return ValidationError{
		Field:  param,
		Reason: reason,
		BaseError: BaseError{
			Error: "path parameter invalid",
		},
	}
}

func NewFieldNotPresentError(field string) ValidationError {
	return ValidationError{
		Field: field,
		BaseError: BaseError{
			Error: "field not present",
		},
	}
}

func NewInvalidField(field string) ValidationError {
	return ValidationError{
		Field: field,
		BaseError: BaseError{
			Error: "invalid data in field",
		},
	}
}

func NewFieldValidationError(field string, reason string) ValidationError {
	return ValidationError{
		Field: field,
		BaseError: BaseError{
			Error: reason,
		},
	}
}

// ConflictsError is returned in the body of an HTTP 409
type ConflictsError struct {
	ID string `json:"id" example:"a1fae5de-dd96-4b20-8362-95f6a574c4b1"`
	BaseError
}

func NewConflictsError(id string) ConflictsError {
	return ConflictsError{
		ID: id,
		BaseError: BaseError{
			Error: "resource already exists",
		},
	}
}

// NotFoundError is returned in the body of an HTTP 404
type NotFoundError struct {
	BaseError
	Resource string `json:"resource,omitempty"`
}

func NewNotFoundError(resource string) NotFoundError {
	return NotFoundError{
		Resource: resource,
		BaseError: BaseError{
			Error: "not found",
		},
	}
}

// NotAllowedError is returned in the body of an HTTP 403
type NotAllowedError struct {
	BaseError
	Reason string `json:"reason,omitempty"`
}

func NewNotAllowedError(reason string) NotAllowedError {
	return NotAllowedError{
		Reason: reason,
		BaseError: BaseError{
			Error: "operation not allowed",
		},
	}
}
