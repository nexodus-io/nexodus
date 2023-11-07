Feature: Device API

  Background:
    Given a user named "Bob" with password "testpass"
    Given a user named "Alice" with password "testpass"

  Scenario: Bob creates and uses a reg-key.

    Given I am logged in as "Bob"

    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${user_id}
    And the response should match json:
      """
      {
        "id": "${user_id}",
        "username": "${response.username}"
      }
      """
    
    When I GET path "/api/vpcs"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${vpc_id}

    # Bob gets an empty list of reg-keys.
    When I GET path "/api/reg-keys"
    Then the response code should be 200
    And the response should match json:
      """
      []
      """

    # Bob creates a reg-key
    When I POST path "/api/reg-keys" with json body:
      """
      {
        "owner_id": "${user_id}",
        "vpc_id": "${vpc_id}"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${reg_token_id}
    Given I store the ".bearer_token" selection from the response as ${reg_bearer_token}
    And the response should match json:
      """
      {
        "id": "${reg_token_id}",
        "bearer_token": "${reg_bearer_token}",
        "owner_id": "${user_id}",
        "vpc_id": "${vpc_id}"
      }
      """

    # Bob gets an should see 1 device in the device listing..
    When I GET path "/api/reg-keys"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "id": "${reg_token_id}",
          "bearer_token": "${reg_bearer_token}",
          "owner_id": "${user_id}",
          "vpc_id": "${vpc_id}"
        }
      ]
      """

    #
    # Verify Alice can't see Bob's stuff
    #
    Given I am logged in as "Alice"

    # Alice gets an empty list of devices..
    When I GET path "/api/reg-keys"
    Then the response code should be 200
    And the response should match json:
      """
      []
      """

    When I GET path "/api/reg-keys/${reg_token_id}"
    Then the response code should be 404

    When I PATCH path "/api/reg-keys/${reg_token_id}" with json body:
      """
      {
        "description": "evilkitten"
      }
      """
    Then the response code should be 404

    When I DELETE path "/api/reg-keys/${reg_token_id}"
    Then the response code should be 404
    And the response should match json:
      """
      {
        "error": "not found",
        "resource": "reg key"
      }
      """

    #
    # Use the device registration bearer_token to create a new device
    #
    Given I set the "Authorization" header to "Bearer ${reg_bearer_token}"

    Given I generate a new key pair as ${private_key}/${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "owner_id": "${user_id}",
        "vpc_id": "${vpc_id}",
        "public_key": "${public_key}",
        "endpoints": [{
          "source": "local",
          "address": "172.17.0.3:58664"
        }, {
          "source": "stun:stun1.l.google.com:19302",
          "address": "172.17.0.3:58664"
        }],
        "tunnel_ip": "",
        "tunnel_ip_v6": "",
        "advertise_cidrs": null,
        "relay": false,
        "symmetric_nat": true,
        "hostname": "bbac3081d5e8",
        "os": "linux"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${device_id}
    Given I store the ".bearer_token" selection from the response as ${device_bearer_token}
    Given I store the ${response} as ${device}

    # bearer_token field will be different every you get the device, so remove it from the comparison
    Given I delete the ${device} "bearer_token" key

    When I GET path "/api/devices/${device_id}"
    Then the response code should be 200
    And the response should contain json:
      """
      ${device}
      """

    #
    # Use the device bearer_token to call apis nexd uses to reconcile it's state.
    #
    Given I decrypt the sealed "${device_bearer_token}" with "${private_key}" and store the result as ${device_bearer_token}
    Given I set the "Authorization" header to "Bearer ${device_bearer_token}"

    When I GET path "/api/devices"
    Then the response code should be 200
    And the ${response[0]} should contain json:
      """
      ${device}
      """

    When I GET path "/api/devices/${device_id}"
    Then the response code should be 200
    And the response should contain json:
      """
      ${device}
      """

    When I GET path "/api/vpcs/${vpc_id}/devices"
    Then the response code should be 200
    And the ${response[0]} should contain json:
      """
      ${device}
      """

    When I POST path "/api/vpcs/${vpc_id}/events" with json body expecting a json event stream:
      """
      [
        {
          "kind": "device",
          "gt_revision": 0
        }
      ]
      """
    Then the response code should be 200
    And the response header "Content-Type" should match "application/json;stream=watch"

    Given I wait up to "3" seconds for a response event
    Then the response should contain json:
      """
      {
        "kind": "device",
        "type": "change",
        "value": ${device}
      }
      """

    #
    # Switch back to Bob, and make sure he can delete his reg-key.
    #
    Given I am logged in as "Bob"
    When I DELETE path "/api/reg-keys/${reg_token_id}"
    Then the response code should be 200
    And the response should match json:
      """
      {
        "id": "${reg_token_id}",
        "bearer_token": "${reg_bearer_token}",
        "owner_id": "${user_id}",
        "vpc_id": "${vpc_id}"
      }
      """

    # Using the token should not work anymore..
    Given I set the "Authorization" header to "Bearer ${reg_bearer_token}"

    When I GET path "/api/devices/${device_id}"
    Then the response code should be 401
    And the response should match json:
      """
      {
        "error": "invalid reg key"
      }
      """

    #
    # the device token should still work..
    Given I set the "Authorization" header to "Bearer ${device_bearer_token}"

    When I GET path "/api/devices"
    Then the response code should be 200
    And the ${response[0]} should contain json:
      """
      ${device}
      """

    #
    # if you delete the device, the device token should not work anymore..
    Given I am logged in as "Bob"
    When I DELETE path "/api/devices/${device_id}"
    Then the response code should be 200

    Given I set the "Authorization" header to "Bearer ${device_bearer_token}"
    When I GET path "/api/devices"
    Then the response code should be 401
    And the response should match json:
      """
      {
        "error": "invalid device token"
      }
      """

  Scenario: Negative Tests

    Given I am logged in as "Bob"

    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${user_id}

    When I POST path "/api/reg-keys" with json body:
      """
      {
        "owner_id": "${user_id}",
        "vpc_id": "test"
      }
      """
    Then the response code should be 400
    And the response should match json:
      """
      {
          "error": "request json is invalid: invalid UUID length: 4"
      }
      """