Feature: Invitations API
  Background:
    Given a user named "Russel" with password "testpass"
    Given a user named "Brent" with password "testpass"
    Given a user named "Anil" with password "testpass"
    Given a user named "EvilBob" with password "testpass"

  Scenario: Invite a user to an organization by user id

    #
    # Get the user and default org ids for two users...
    #
    Given I am logged in as "Russel"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ${response} as ${russel_user}
    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0]} as ${russel_organization}

    Given I am logged in as "Anil"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ${response} as ${anil_user}
    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0]} as ${anil_organization}

    Given I am logged in as "Brent"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ${response} as ${brent_user}
    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0]} as ${brent_organization}

    # There is only one user in the brent org:
    When I GET path "/api/organizations/${brent_organization.id}/users"
    Then the response code should be 200
    And the response should match json:
      """
      [{
          "organization_id": "${brent_organization.id}",
          "user_id": "${brent_user.id}",
          "user": ${brent_user}
      }]
      """

    # Brent invites Russel and Anil to his org:
    When I POST path "/api/invitations" with json body:
      """
      {
        "user_id": "${russel_user.id}",
        "organization_id": "${brent_organization.id}"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${invitation_id}

    When I POST path "/api/invitations" with json body:
      """
      {
        "user_id": "${anil_user.id}",
        "organization_id": "${brent_organization.id}"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${anil_invitation_id}

    Given I am logged in as "Anil"
    When I POST path "/api/invitations/${anil_invitation_id}/accept"
    Then the response code should be 204
    And the response should match ""

    Given I am logged in as "Russel"

    # Russel can't see Brent's org members yet.
    When I GET path "/api/organizations/${brent_organization.id}/users"
    Then the response code should be 404
    And the response should match json:
      """
      {
        "error": "not found",
        "resource": "organization"
      }
      """

    # Accept the invitation.
    When I POST path "/api/invitations/${invitation_id}/accept"
    Then the response code should be 204
    And the response should match ""

    # Russel should be in two orgs now...
    When I GET path "/api/organizations"
    Then the response code should be 200
    And the response should match json:
      """
      [ ${brent_organization}, ${russel_organization} ]
      """

    # Russel can now see Brent's org members yet.
    When I GET path "/api/organizations/${brent_organization.id}/users"
    Then the response code should be 200
    And the response should match json:
      """
      [{
          "organization_id": "${brent_organization.id}",
          "user_id": "${anil_user.id}",
          "user": ${anil_user}
      }, {
          "organization_id": "${brent_organization.id}",
          "user_id": "${brent_user.id}",
          "user": ${brent_user}
      }, {
          "organization_id": "${brent_organization.id}",
          "user_id": "${russel_user.id}",
          "user": ${russel_user}
      }]
      """

    # Russel can't modify Brent's org members.
    When I DELETE path "/api/organizations/${brent_organization.id}/users/${anil_user.id}"
    Then the response code should be 404
      """
      {
        "error": "not found",
        "resource": "organization"
      }
      """

    # Switch to the Org owner now....
    Given I am logged in as "Brent"

    # You can't delete org owner from the org.
    When I DELETE path "/api/organizations/${brent_organization.id}/users/${brent_user.id}"
    Then the response code should be 400
    And the response should match json:
      """
      {
        "error": "path parameter invalid",
        "field": "uid",
        "reason": "cannot delete owner of the organization"
      }
      """

    When I GET path "/api/organizations/${brent_organization.id}/users/${anil_user.id}"
    Then the response code should be 200
    And the response should match json:
      """
      {
          "organization_id": "${brent_organization.id}",
          "user_id": "${anil_user.id}",
          "user": ${anil_user}
      }
      """

    # Brent can modify his org members.
    When I DELETE path "/api/organizations/${brent_organization.id}/users/${anil_user.id}"
    Then the response code should be 200
    And the response should match json:
      """
      {
          "organization_id": "${brent_organization.id}",
          "user_id": "${anil_user.id}",
          "user": ${anil_user}
      }
      """

    When I GET path "/api/organizations/${brent_organization.id}/users"
    Then the response code should be 200
    And the response should match json:
      """
      [{
          "organization_id": "${brent_organization.id}",
          "user_id": "${brent_user.id}",
          "user": ${brent_user}
      }, {
          "organization_id": "${brent_organization.id}",
          "user_id": "${russel_user.id}",
          "user": ${russel_user}
      }]
      """