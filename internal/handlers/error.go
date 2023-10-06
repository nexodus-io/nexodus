package handlers

import (
	"encoding/json"
)

type ApiResponseError struct {
	Status int
	Body   any
}

func (e ApiResponseError) Error() string {
	data, err := json.Marshal(e.Body)
	if err != nil {
		return "ApiResponseError"
	}
	return string(data)
}

func NewApiResponseError(status int, body any) *ApiResponseError {
	return &ApiResponseError{
		Status: status,
		Body:   body,
	}
}
