package util

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func isMaxRetriesReachedErr(err error) bool {
	return strings.Contains(err.Error(), "giving up after")
}

func TestRetryOperation(t *testing.T) {
	tests := []struct {
		name           string
		wait           time.Duration
		retries        int
		operation      func() error
		expectedResult error
	}{
		{
			name:    "Success on first attempt",
			wait:    10 * time.Millisecond,
			retries: 3,
			operation: func() error {
				return nil
			},
			expectedResult: nil,
		},
		{
			name:    "Success after retries",
			wait:    10 * time.Millisecond,
			retries: 3,
			operation: func() error {
				return errors.New("temporary error")
			},
			expectedResult: nil,
		},
		{
			name:    "Context canceled",
			wait:    50 * time.Millisecond,
			retries: 3,
			operation: func() error {
				return errors.New("temporary error")
			},
			expectedResult: context.Canceled,
		},
	}

	callCount := 0
	for _, tt := range tests {
		callCount = 0 // Reset the callCount before each test
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			if tt.name == "Context canceled" {
				go func() {
					time.Sleep(25 * time.Millisecond)
					cancel()
				}()
			}

			// Update the "Success after retries" test case to use the callCount variable
			if tt.name == "Success after retries" {
				tt.operation = func() error {
					if callCount < 2 { // Assuming it will succeed after 2 retries (total 3 attempts)
						callCount++
						return errors.New("temporary error")
					}
					return nil
				}
			}

			err := RetryOperation(ctx, tt.wait, tt.retries, tt.operation)
			if tt.expectedResult == nil {
				assert.Nil(t, err)
			} else if errors.Is(err, context.Canceled) {
				assert.Equal(t, tt.expectedResult, err)
			} else {
				assert.True(t, isMaxRetriesReachedErr(err))
			}
		})
	}
}

func TestRetryOperationForErrors(t *testing.T) {
	temporaryErr := errors.New("temporary error")
	nonRetriableError := errors.New("non-retriable error")
	tests := []struct {
		name           string
		wait           time.Duration
		retries        int
		operation      func() error
		expectedResult error
	}{
		{
			name:    "Success on first attempt",
			wait:    10 * time.Millisecond,
			retries: 3,
			operation: func() error {
				return nil
			},
			expectedResult: nil,
		},
		{
			name:    "Success after retries",
			wait:    10 * time.Millisecond,
			retries: 3,
			operation: func() error {
				return temporaryErr
			},
			expectedResult: nil,
		},
		{
			name:    "Non-retriable error",
			wait:    10 * time.Millisecond,
			retries: 3,
			operation: func() error {
				return nonRetriableError
			},
			expectedResult: nonRetriableError,
		},
		{
			name:    "Context canceled",
			wait:    50 * time.Millisecond,
			retries: 3,
			operation: func() error {
				return temporaryErr
			},
			expectedResult: context.Canceled,
		},
	}

	callCount := 0
	for _, tt := range tests {
		callCount = 0 // Reset the callCount before each test
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			if tt.name == "Context canceled" {
				go func() {
					time.Sleep(25 * time.Millisecond)
					cancel()
				}()
			}

			// Update the "Success after retries" test case to use the callCount variable
			if tt.name == "Success after retries" {
				tt.operation = func() error {
					if callCount < 2 { // Assuming it will succeed after 2 retries (total 3 attempts)
						callCount++
						return temporaryErr
					}
					return nil
				}
			}

			err := RetryOperationForErrors(ctx, tt.wait, tt.retries, []error{temporaryErr}, tt.operation)
			if tt.expectedResult == nil {
				assert.Nil(t, err)
			} else if errors.Is(err, nonRetriableError) {
				// We should expect the error to be returned as-is and the call count to not have increased
				assert.Equal(t, tt.expectedResult, err)
				assert.Equal(t, 0, callCount)
			} else if errors.Is(err, context.Canceled) {
				assert.Equal(t, tt.expectedResult, err)
			} else {
				assert.True(t, isMaxRetriesReachedErr(err))
			}
		})
	}
}
