package models

type ApiError struct {
	Error string `json:"error" example:"something bad"`
}
