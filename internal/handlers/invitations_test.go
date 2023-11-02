package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
)

func (suite *HandlerTestSuite) TestCreateAcceptRefuseInvitation() {
	require := suite.Require()

	var inviteID uuid.UUID

	tt := []struct {
		name   string
		userID uuid.UUID
		orgID  uuid.UUID
		code   int
		action string
		login  uuid.UUID
	}{
		{
			name:   "invite to existing org fails",
			code:   http.StatusBadRequest,
			userID: TestUserID,
			orgID:  suite.testOrganizationID,
			action: "invite",
		},
		{
			login:  TestUser2ID,
			name:   "invite to new org succeeds",
			code:   http.StatusCreated,
			userID: TestUserID,
			orgID:  suite.testUser2OrgID,
			action: "invite",
		},
		{
			login:  TestUser2ID,
			name:   "re-invite to same org fails",
			code:   http.StatusConflict,
			userID: TestUserID,
			orgID:  suite.testUser2OrgID,
			action: "invite",
		},
		{
			name:   "refuse invite succeeds",
			code:   http.StatusNoContent,
			userID: TestUserID,
			orgID:  suite.testUser2OrgID,
			action: "refuse",
		},
		{
			login:  TestUser2ID,
			name:   "re-invite to same org succeeds",
			code:   http.StatusCreated,
			userID: TestUserID,
			orgID:  suite.testUser2OrgID,
			action: "invite",
		},
		{
			name:   "accept org succeeds",
			code:   http.StatusNoContent,
			userID: TestUserID,
			orgID:  suite.testUser2OrgID,
			action: "accept",
		},
		{
			name:   "invite to existing org fails",
			code:   http.StatusBadRequest,
			userID: TestUserID,
			orgID:  suite.testOrganizationID,
			action: "invite",
		},
	}
	for _, c := range tt {
		suite.T().Log(c.name)

		var res *httptest.ResponseRecorder
		switch c.action {
		case "invite":
			request := models.AddInvitation{
				UserID:         &c.userID,
				OrganizationID: c.orgID,
			}
			reqBody, err := json.Marshal(request)
			require.NoError(err)
			_, res, err = suite.ServeRequest(
				http.MethodPost,
				"/", "/",
				func(ctx *gin.Context) {
					ctx.Set("_apex.testCreateOrganization", "true")
					if c.userID != uuid.Nil {
						ctx.Set(gin.AuthUserKey, c.login)
					}
					suite.api.CreateInvitation(ctx)
				}, bytes.NewReader(reqBody),
			)
			require.NoError(err)
			body, err := io.ReadAll(res.Body)
			require.NoError(err)
			require.Equal(c.code, res.Code, "HTTP error: %s", string(body))
			if res.Code == http.StatusCreated {
				var inv models.Invitation
				err = json.Unmarshal(body, &inv)
				require.NoError(err)
				require.NotEqual(uuid.Nil, inv.ID)
				inviteID = inv.ID
			}
		case "accept":
			var err error
			require.NotEqual(uuid.Nil, inviteID)
			_, res, err = suite.ServeRequest(
				http.MethodPost,
				"/:invitation", fmt.Sprintf("/%s", inviteID.String()),
				func(c *gin.Context) {
					c.Set("_apex.testCreateOrganization", "true")
					suite.api.AcceptInvitation(c)
				}, nil,
			)
			require.NoError(err)
			body, err := io.ReadAll(res.Body)
			require.NoError(err)
			require.Equal(c.code, res.Code, "HTTP error: %s", string(body))
		case "refuse":
			var err error
			require.NotEqual(uuid.Nil, inviteID)
			_, res, err = suite.ServeRequest(
				http.MethodPost,
				"/:invitation", fmt.Sprintf("/%s", inviteID.String()),
				func(c *gin.Context) {
					c.Set("_apex.testCreateOrganization", "true")
					suite.api.DeleteInvitation(c)
				}, nil,
			)
			require.NoError(err)
			body, err := io.ReadAll(res.Body)
			require.NoError(err)
			require.Equal(c.code, res.Code, "HTTP error: %s", string(body))
		}
	}
}
