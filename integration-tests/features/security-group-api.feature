Feature: Security Group API

  Background:
    Given a user named "Oscar" with password "testpass"
    Given a user named "EvilBob" with password "testpass"

  Scenario: Show basic security group api in action

    Given I am logged in as "Oscar"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oscar_user_id}

    # get the default security group.. it's ID should match the user's ID
    When I GET path "/api/security-groups/${oscar_user_id}"
    Then the response code should be 200
    And the response should match json:
      """
      {
        "group_description": "default organization security group",
        "group_name": "default",
        "id": "${oscar_user_id}",
        "organization_id": "${oscar_user_id}",
        "revision": ${response.revision}
      }
      """
    Given I store the ${response} as ${default_security_group}

    # Oscar should only have one security group at this point.
    When I GET path "/api/security-groups"
    Then the response code should be 200
    And the response should match json:
        """
        [ ${default_security_group} ]
        """

    # He can create additional security groups
    When I POST path "/api/security-groups" with json body:
      """
      {
        "organization_id": "${oscar_user_id}",
        "group_description": "extra security group",
        "group_name": "extra"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${extra_security_group_id}
    And the response should match json:
      """
      {
        "id": "${extra_security_group_id}",
        "organization_id": "${oscar_user_id}",
        "group_description": "extra security group",
        "group_name": "extra",
        "revision": ${response.revision}
      }
      """
    Given I store the ${response} as ${extra_security_group}

    # Oscar should now have two security groups.
    When I GET path "/api/security-groups"
    Then the response code should be 200
    And the response should match json:
      """
      [ ${default_security_group}, ${extra_security_group} ]
      """

    # let's verify another user cannot access any of Oscar's resources
    Given I am logged in as "EvilBob"
    When I GET path "/api/security-groups/${oscar_user_id}"
    Then the response code should be 404
    When I GET path "/api/security-groups/${extra_security_group_id}"
    Then the response code should be 404
    When I DELETE path "/api/security-groups/${oscar_user_id}"
    Then the response code should be 404
    When I DELETE path "/api/security-groups/${extra_security_group_id}"
    Then the response code should be 404

    # Switch back to Oscar
    Given I am logged in as "Oscar"

    # We should be able to delete non default security groups.
    When I DELETE path "/api/security-groups/${extra_security_group_id}"
    Then the response code should be 200
    And the response should match json:
      """
      ${extra_security_group}
      """

    # The organization's security-group should be the default security group....
    When I GET path "/api/organizations/${oscar_user_id}"
    Then the response code should be 200
    And the response should match json:
      """
      {
        "id": "${oscar_user_id}",
        "name": "${response.name}",
        "description": "${response.description}",
        "owner_id": "${oscar_user_id}",
        "security_group_id": "${oscar_user_id}"
      }
      """

    # If we delete the default security group, it should removed from the organization.
    When I DELETE path "/api/security-groups/${oscar_user_id}"
    Then the response code should be 200
    And the response should match json:
      """
      ${default_security_group}
      """

    # verify it gets removed...
    When I GET path "/api/organizations/${oscar_user_id}"
    Then the response code should be 200
    And the response should match json:
      """
      {
        "id": "${oscar_user_id}",
        "name": "${response.name}",
        "description": "${response.description}",
        "owner_id": "${oscar_user_id}",
        "security_group_id": "00000000-0000-0000-0000-000000000000"
      }
      """
