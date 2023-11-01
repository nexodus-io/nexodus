Feature: Security Group API
  Background:
    Given a user named "Oscar" with password "testpass"

  Scenario: Show basic security group api in action

    #
    # Get the user and default org ids for a user...
    #

    Given I am logged in as "Oscar"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oscar_user_id}
    Given I store the ".username" selection from the response as ${oscar_username}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${oscar_organization_id}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].security_group_id} as ${oscar_security_group_id}

    When I GET path "/api/organizations"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "cidr": "100.64.0.0/10",
          "cidr_v6": "200::/64",
          "description": "${oscar_username}'s organization",
          "hub_zone": true,
          "id": "${oscar_organization_id}",
          "name": "${oscar_username}",
          "owner_id": "${oscar_user_id}",
          "security_group_id": "${oscar_security_group_id}",
          "private_cidr": false
        }
      ]
      """

    When I GET path "/api/organizations/${oscar_organization_id}/security_group/${oscar_security_group_id}"
    Then the response code should be 200
    Given I store the ".revision" selection from the response as ${current_revision}
    And the response should match json:
  """
      {
        "group_description": "default organization security group",
        "group_name": "default",
        "id": "${oscar_security_group_id}",
        "organization_id": "${oscar_organization_id}",
        "revision": ${current_revision}
      }
  """
