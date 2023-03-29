Feature: Organization API
  Background:
    Given a user named "Oliver" with password "testpass"
    Given a user named "Oscar" with password "testpass"
    Given a user named "EvilBob" with password "testpass"

  Scenario: Show basic organization api in action

    #
    # Get the user and default org ids for two users...
    #
    Given I am logged in as "Oliver"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".organizations[0]" selection from the response as ${oliver_organization_id}
    Given I store the ".id" selection from the response as ${oliver_user_id}

    Given I am logged in as "Oscar"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".organizations[0]" selection from the response as ${oscar_organization_id}
    Given I store the ".id" selection from the response as ${oscar_user_id}
    Given I store the ".username" selection from the response as ${oscar_username}

    #
    # Oscar should only be able to see the orgs that he is a part of.
    When I GET path "/api/organizations"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "cidr": "100.100.0.0/16",
          "description": "${oscar_username}'s organization",
          "devices": [],
          "hub_zone": true,
          "id": "${oscar_organization_id}",
          "name": "${oscar_username}",
          "owner_id": "${oscar_user_id}",
          "users": [
            "${oscar_user_id}"
          ]
        }
      ]
      """
