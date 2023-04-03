Feature: Organization Devices API

  Background:
    Given a user named "Oliver" with password "testpass"
    Given a user named "Oscar" with password "testpass"
    Given a user named "EvilBob" with password "testpass"

  Scenario: Show basic organization devices api in action

    Given I am logged in as "EvilBob"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${evilbob_user_id}

    #
    # Lets puts add Oliver in Oscar's org.
    #
    Given I am logged in as "Oliver"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oliver_user_id}

    Given I am logged in as "Oscar"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oscar_user_id}

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

    # Each user can create devices in the org:
    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${oliver_user_id}",
        "organization_id": "${organization_id}",
        "public_key": "${public_key}",
        "local_ip": "172.17.0.3:58664",
        "tunnel_ip": "",
        "child_prefix": null,
        "relay": false,
        "discovery": false,
        "reflexive_ip4": "47.196.141.165",
        "endpoint_local_address_ip4": "172.17.0.3",
        "symmetric_nat": true,
        "hostname": "oliver-laptop"
      }
      """
    Then the response code should be 201
    Given I store the ${response} as ${oliver_device}

    Given I am logged in as "Oscar"
    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${oscar_user_id}",
        "organization_id": "${organization_id}",
        "public_key": "${public_key}",
        "local_ip": "172.17.0.1:58664",
        "tunnel_ip": "",
        "child_prefix": null,
        "relay": false,
        "discovery": false,
        "reflexive_ip4": "47.196.141.164",
        "endpoint_local_address_ip4": "172.17.0.2",
        "symmetric_nat": true,
        "hostname": "oscar-pc"
      }
      """
    Then the response code should be 201
    Given I store the ${response} as ${oscar_device}

    # Users that are not in the org should not be able to add devices to the org.
    Given I am logged in as "EvilBob"
    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${evilbob_user_id}",
        "organization_id": "${organization_id}",
        "public_key": "${public_key}",
        "local_ip": "172.17.0.21:58664",
        "tunnel_ip": "",
        "child_prefix": null,
        "relay": false,
        "discovery": false,
        "reflexive_ip4": "47.196.141.124",
        "endpoint_local_address_ip4": "172.17.2.2",
        "symmetric_nat": true,
        "hostname": "bob-pc"
      }
      """
    Then the response code should be 404
    And the response should match json:
      """
      { "error": "operation not allowed", "reason": "user or organization" }
      """

    # They both can see the devices
    Given I am logged in as "Oscar"
    When I GET path "/api/organizations/${organization_id}/devices"
    Then the response code should be 200
    And the response should match json:
      """
      [
        ${oliver_device},
        ${oscar_device}
      ]
      """

    Given I am logged in as "Oliver"
    When I GET path "/api/organizations/${organization_id}/devices"
    Then the response code should be 200
    And the response should match json:
      """
      [
        ${oliver_device},
        ${oscar_device}
      ]
      """

    # Other user's should not see the devices.
    Given I am logged in as "EvilBob"
    When I GET path "/api/organizations/${organization_id}/devices"
    Then the response code should be 404
    And the response should match json:
      """
      { "error": "not found", "resource": "organization" }
      """