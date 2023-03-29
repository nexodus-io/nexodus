package handlers

import (
	"context"
)

func (api *API) populateStore(parent context.Context) error {
	_, span := tracer.Start(parent, "populateStore")
	defer span.End()

	return nil
}
