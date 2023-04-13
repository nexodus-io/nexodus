Feature: Organization Users API

  Background:
    Given a user named "Oliver" with password "testpass"
    Given a user named "Oscar" with password "testpass"
    Given a user named "EvilBob" with password "testpass"

  Scenario: Show basic organization users api in action

    Given I am logged in as "EvilBob"
    When I GET path "/api/users/me"
    Then the response code should be 200

    #
    # Lets puts add Oliver in Oscar's org.
    #
    Given I am logged in as "Oliver"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oliver_user_id}
    Given I store the ${response} as ${oliver_user}

    Given I am logged in as "Oscar"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oscar_user_id}
    Given I store the ${response} as ${oscar_user}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${organization_id}

    When I POST path "/api/invitations" with json body:
      """
      {
        "user_id": "${oliver_user_id}",
        "organization_id": "${organization_id}"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${invitation_id}

    Given I am logged in as "Oliver"
    When I POST path "/api/invitations/${invitation_id}/accept"
    Then the response code should be 204
    And the response should match ""

    # They both can see the org users
    Given I am logged in as "Oscar"
    When I GET path "/api/organizations/${organization_id}/users"
    Then the response code should be 200
    And the response should match json:
      """
      [
        ${oliver_user},
        ${oscar_user}
      ]
      """

    Given I am logged in as "Oliver"
    When I GET path "/api/organizations/${organization_id}/users"
    Then the response code should be 200
    And the response should match json:
      """
      [
        ${oliver_user},
        ${oscar_user}
      ]
      """

    # Other user's should not see the devices.
    Given I am logged in as "EvilBob"
    When I GET path "/api/organizations/${organization_id}/users"
    Then the response code should be 404
    And the response should match json:
      """
      { "error": "not found", "resource": "organization" }
      """