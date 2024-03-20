package client

import (
	"errors"
	"fmt"
	"net/http"
)

func Simplify[T any](resp T, httpResp *http.Response, err error) (T, error) {
	if err != nil {
		statusSuffix := ""
		if httpResp != nil {
			statusSuffix = fmt.Sprintf(", status: %d", httpResp.StatusCode)
		}
		var openAPIError *GenericOpenAPIError
		switch {
		case errors.As(err, &openAPIError):
			model := openAPIError.Model()
			switch err := model.(type) {
			case ModelsBaseError:
				return resp, fmt.Errorf("error: %s%s", err.GetError(), statusSuffix)
			case ModelsConflictsError:
				return resp, fmt.Errorf("error: %s: conflicting id: %s%s", err.GetError(), err.GetId(), statusSuffix)
			case ModelsNotAllowedError:
				message := fmt.Sprintf("error: %s", err.GetError())
				if err.Reason != nil {
					message += fmt.Sprintf(", reason: %s", err.GetReason())
				}
				message += statusSuffix
				return resp, fmt.Errorf(message)
			case ModelsValidationError:
				message := fmt.Sprintf("error: %s", err.GetError())
				if err.Field != nil {
					message += fmt.Sprintf(", field: %s", err.GetField())
				}
				message += statusSuffix
				return resp, fmt.Errorf(message)
			case ModelsInternalServerError:
				return resp, fmt.Errorf("error: %s: trace id: %s%s", err.GetError(), err.GetTraceId(), statusSuffix)
			default:
				return resp, fmt.Errorf("error: %s%s", string(openAPIError.Body()), statusSuffix)
			}
		default:
			return resp, fmt.Errorf("error: %w%s", err, statusSuffix)
		}
	}
	return resp, nil
}
