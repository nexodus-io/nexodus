Feature: Organization API
  Background:
    Given a user named "Oscar" with password "testpass"
    Given a user named "EvilBob" with password "testpass"

  Scenario: Show basic vpc api in action

    #
    # Get the user and default org ids for two users...
    #

    Given I am logged in as "Oscar"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oscar_user_id}
    Given I store the ".username" selection from the response as ${oscar_username}

    #
    # Oscar's default vpc should have the same id as the user id.
    When I GET path "/api/vpcs/${oscar_user_id}"
    Then the response code should be 200
    And the response should match json:
      """
      {
        "ipv4_cidr": "100.64.0.0/10",
        "ipv6_cidr": "200::/64",
        "description": "default vpc",
        "id": "${oscar_user_id}",
        "organization_id": "${oscar_user_id}",
        "private_cidr": false
      }
      """
    Given I store the ${response} as ${default_vpc}

    #
    # The default vpc should be the only listed vpc
    When I GET path "/api/vpcs"
    Then the response code should be 200
    And the response should match json:
      """
      [
         ${default_vpc}
      ]
      """

    # Oscar should not be able to delete his default VPC
    When I DELETE path "/api/vpcs/${oscar_user_id}"
    Then the response code should be 400
    And the response should match json:
      """
      {
        "error": "operation not allowed",
        "reason": "default vpc cannot be deleted"
      }
      """

    # But we can create additional VPCs
    When I POST path "/api/vpcs" with json body:
      """
      {
        "organization_id": "${oscar_user_id}",
        "description": "extra vpc"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${extra_vpc_id}
    And the response should match json:
      """
      {
        "ipv4_cidr": "100.64.0.0/10",
        "ipv6_cidr": "200::/64",
        "description": "extra vpc",
        "id": "${extra_vpc_id}",
        "organization_id": "${oscar_user_id}",
        "private_cidr": false
      }
      """
    Given I store the ${response} as ${extra_vpc}

    # It should be added to the list of the vpc the user can see...
    When I GET path "/api/vpcs"
    Then the response code should be 200
    And the response should match json:
      """
      [
         ${default_vpc},
         ${extra_vpc}
      ]
      """

    # We can modify the description of the extra vpc
    When I PATCH path "/api/vpcs/${extra_vpc_id}" with json body:
      """
      {
        "description": "extra vpc modified"
      }
      """
    Then the response code should be 200
    And the response should match json:
      """
      {
        "ipv4_cidr": "100.64.0.0/10",
        "ipv6_cidr": "200::/64",
        "description": "extra vpc modified",
        "id": "${extra_vpc_id}",
        "organization_id": "${oscar_user_id}",
        "private_cidr": false
      }
      """
    Then I store the ${response} as ${extra_vpc}

    # let's verify another user cannot access any of Oscar's resources
    Given I am logged in as "EvilBob"
    When I GET path "/api/vpcs/${oscar_user_id}"
    Then the response code should be 404
    When I GET path "/api/vpcs/${extra_vpc_id}"
    Then the response code should be 404
    When I DELETE path "/api/vpcs/${oscar_user_id}"
    Then the response code should be 404
    When I DELETE path "/api/vpcs/${extra_vpc_id}"
    Then the response code should be 404

    # Switch back to Oscar
    Given I am logged in as "Oscar"

    # Verify VPCs cannot be deleted when they have a device attached
    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${oscar_user_id}",
        "vpc_id": "${extra_vpc_id}",
        "public_key": "${public_key}",
        "hostname": "device1"
      }
      """
    Then the response code should be 201
    Given I store the ${response.id} as ${device_id}

    When I DELETE path "/api/vpcs/${extra_vpc_id}"
    Then the response code should be 400
    And the response should match json:
      """
      {
        "error": "operation not allowed",
        "reason": "vpc cannot be deleted while devices are still attached"
      }
      """

    # Now lets delete the device and try again
    When I DELETE path "/api/devices/${device_id}"
    Then the response code should be 200

    When I DELETE path "/api/vpcs/${extra_vpc_id}"
    Then the response code should be 200
    And the response should match json:
      """
      ${extra_vpc}
      """

  Scenario: Bad Requests

    Given I am logged in as "Oscar"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oscar_user_id}

    # bad json
    When I POST path "/api/vpcs" with json body:
      """
      {
        "organization_id": "${oscar_user_id}",
      }
      """
    Then the response code should be 400
    And the response should match json:
      """
      {
        "error": "request json is invalid: invalid character '}' looking for beginning of object key string"
      }
      """

    # bad ipv4_cidr
    When I POST path "/api/vpcs" with json body:
      """
      {
        "organization_id": "${oscar_user_id}",
        "private_cidr": true,
        "ipv4_cidr": "100.64.0.0//10",
        "ipv6_cidr": "200::/64"
      }
      """
    Then the response code should be 400
    And the response should match json:
      """
      {
        "error": "invalid CIDR address: 100.64.0.0//10",
        "field": "ipv4_cidr"
      }
      """

    # bad ipv6_cidr
    When I POST path "/api/vpcs" with json body:
      """
      {
        "organization_id": "${oscar_user_id}",
        "private_cidr": true,
        "ipv4_cidr": "100.64.0.0/10",
        "ipv6_cidr": "200:://64"
      }
      """

    Then the response code should be 400
    And the response should match json:
      """
      {
        "error": "invalid CIDR address: 200:://64",
        "field": "cidr_v6"
      }
      """
