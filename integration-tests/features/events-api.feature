Feature: Events API

  Background:
    Given a user named "Oliver" with password "testpass"
    Given a user named "Oscar" with password "testpass"
    Given a user named "EvilBob" with password "testpass"

  Scenario: Using the events endpoint to stream device and metadata change events

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

    # Create a device...
    Given I am logged in as "Oscar"
    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${oscar_user_id}",
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
        "hostname": "device1"
      }
      """
    Then the response code should be 201
    Given I store the ${response} as ${device1}
    Given I store the ${response.id} as ${device_id}

    Given I am logged in as "Oliver"

    When I POST path "/api/organizations/${organization_id}/events" with json body expecting a json event stream:
      """
      [
        {
          "kind": "device",
          "gt_revision": 0
        },
        {
          "kind": "device-metadata",
          "gt_revision": 0
        }
      ]
      """
    Then the response code should be 200
    And the response header "Content-Type" should match "application/json;stream=watch"

    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "device-metadata",
        "type": "bookmark"
      }
      """

    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "device",
        "type": "change",
        "value": ${device1}
      }
      """

    # The book mark event signals that you have now received the full list of devices.
    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "device",
        "type": "bookmark"
      }
      """

    # Create another device...
    Given I am logged in as "Oscar"
    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${oscar_user_id}",
        "organization_id": "${organization_id}",
        "public_key": "${public_key}",
        "local_ip": "172.17.0.4:58664",
        "tunnel_ip": "",
        "child_prefix": null,
        "relay": false,
        "discovery": false,
        "reflexive_ip4": "47.196.141.166",
        "endpoint_local_address_ip4": "172.17.0.4",
        "symmetric_nat": true,
        "hostname": "device2"
      }
      """
    Then the response code should be 201
    Given I store the ${response} as ${device2}
    Given I store the ${response.id} as ${device2_id}

    # We should get additional change events...
    Given I am logged in as "Oliver"
    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "device",
        "type": "change",
        "value": ${device2}
      }
      """

    # Set device metadata
    Given I am logged in as "Oscar"
    When I PUT path "/api/devices/${device_id}/metadata/tcp:1024" with json body:
      """
      {
        "address": "backend:8080"
      }
      """
    Then the response code should be 200
    Given I store the ${response} as ${metadata1}

    # We should get additional change events...
    Given I am logged in as "Oliver"
    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "device-metadata",
        "type": "change",
        "value": ${metadata1}
      }
      """

    Given I am logged in as "Oscar"
    When I DELETE path "/api/devices/${device2_id}"
    Then the response code should be 200
    Given I store the ${response} as ${deleted_device}

    Given I am logged in as "Oliver"
    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "device",
        "type": "delete",
        "value": ${deleted_device}
      }
      """

