package handlers

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
)

func (api *API) populateStore(parent context.Context) error {
	ctx, span := tracer.Start(parent, "populateStore")
	defer span.End()
	type userOrgMapping struct {
		UserID         string
		OrganizationID uuid.UUID
	}
	var results []userOrgMapping
	if res := api.db.Table("user_organizations").Select("user_id", "organization_id").Scan(&results); res.Error != nil {
		return res.Error
	}

	for _, res := range results {
		path := []string{"user_org_map", res.UserID, res.OrganizationID.String()}
		if err := storage.Txn(ctx, api.store, storage.WriteParams, func(t storage.Transaction) error {
			if err := storage.MakeDir(ctx, api.store, t, path); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return nil
		}

		if err := storage.WriteOne(ctx, api.store, storage.ReplaceOp, path, true); err != nil {
			return err
		}
	}

	return nil
}

func (api *API) addUserOrgMapping(parent context.Context, userID string, orgID uuid.UUID) error {
	ctx, span := tracer.Start(parent, "addUserOrgMapping")
	defer span.End()

	path := []string{"user_org_map", userID, orgID.String()}
	if err := storage.Txn(ctx, api.store, storage.WriteParams, func(t storage.Transaction) error {
		if err := storage.MakeDir(ctx, api.store, t, path); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil
	}

	if err := storage.WriteOne(ctx, api.store, storage.ReplaceOp, path, true); err != nil {
		return err
	}

	return nil
}

func (api *API) deleteUserOrgMapping(parent context.Context, userID string, orgID uuid.UUID) error {
	ctx, span := tracer.Start(parent, "deleteUserOrgMapping")
	defer span.End()

	path := []string{"user_org_map", userID, orgID.String()}
	if err := storage.WriteOne(ctx, api.store, storage.RemoveOp, path, nil); err != nil {
		return err
	}

	return nil
}

func (api *API) userIsInOrg(ctx context.Context, user string, org string) (bool, error) {
	query, err := rego.New(
		rego.Query(
			fmt.Sprintf(`
			data.user_org_map["%s"]["%s"]`,
				user, org,
			)),
		rego.Store(api.store),
	).PrepareForEval(ctx)
	if err != nil {
		return false, err
	}

	res, err := query.Eval(ctx)
	if err != nil {
		return false, err
	}

	return res.Allowed(), nil
}
