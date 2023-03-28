package models

// BaseError is the base type for API errors
type BaseError struct {
	Error string `json:"error" example:"something bad"`
}

// NewApiInternalError returns a new response body for a HTTP 501
func NewApiInternalError(err error) BaseError {
	return BaseError{
		Error: err.Error(),
	}
}

// ValidationError is returned in the body of an HTTP 400
type ValidationError struct {
	BaseError
	Field string `json:"field,omitempty"`
}

func NewBadPayloadError() ValidationError {
	return ValidationError{
		BaseError: BaseError{
			Error: "request json is invalid",
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
