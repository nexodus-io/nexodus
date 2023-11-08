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
        "description": "default vpc security group",
        "id": "${oscar_user_id}",
        "vpc_id": "${oscar_user_id}",
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
        "vpc_id": "${oscar_user_id}",
        "description": "extra security group"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${extra_security_group_id}
    And the response should match json:
      """
      {
        "id": "${extra_security_group_id}",
        "vpc_id": "${oscar_user_id}",
        "description": "extra security group",
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

    # The default security group cannot be deleted.
    When I DELETE path "/api/security-groups/${oscar_user_id}"
    Then the response code should be 400
    And the response should match json:
      """
      {
        "error": "operation not allowed",
        "reason": "default security group cannot be deleted"
      }
      """

