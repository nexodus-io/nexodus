Feature: Organization API
  Background:
    Given a user named "Oliver" with password "testpass"
    Given a user named "Oscar" with password "testpass"
    Given a user named "EvilBob" with password "testpass"

  Scenario: Show basic vpc api in action

    #
    # Get the user and default org ids for two users...
    #
    Given I am logged in as "Oliver"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oliver_user_id}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${oliver_organization_id}

    When I GET path "/api/vpcs"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${oliver_vpc_id}

    Given I am logged in as "Oscar"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oscar_user_id}
    Given I store the ".username" selection from the response as ${oscar_username}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${oscar_organization_id}

    When I GET path "/api/vpcs"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${oscar_vpc_id}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].security_group_id} as ${oscar_security_group_id}

    #
    # Oscar should only be able to see the orgs that he is a part of.
    When I GET path "/api/organizations"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "description": "${oscar_username}'s organization",
          "id": "${oscar_organization_id}",
          "name": "${oscar_username}",
          "owner_id": "${oscar_user_id}",
          "security_group_id": "${oscar_security_group_id}"
        }
      ]
      """

    #
    # Oscar should only be able to see the vpcs that he is a part of.
    When I GET path "/api/vpcs"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "cidr": "100.64.0.0/10",
          "cidr_v6": "200::/64",
          "description": "default vpc",
          "hub_zone": false,
          "id": "${oscar_organization_id}",
          "organization_id": "${oscar_organization_id}",
          "private_cidr": false
        }
      ]
      """
