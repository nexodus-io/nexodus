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

    When I GET path "/api/vpcs"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${vpc_id}

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

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${oscar_organization_id}
    Given I store the ${response[0].security_group_id} as ${oscar_security_group_id}

    When I GET path "/api/security-groups/${oscar_security_group_id}"
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
    Given I store the ${response} as ${security_group}

    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${oscar_user_id}",
        "vpc_id": "${vpc_id}",
        "public_key": "${public_key}",
        "local_ip": "172.17.0.3:58664",
        "tunnel_ip": "",
        "advertise_cidrs": null,
        "relay": false,
        "symmetric_nat": true,
        "hostname": "device1"
      }
      """
    Then the response code should be 201
    Given I store the ${response} as ${device1}
    Given I store the ${response.id} as ${device_id}

    Given I am logged in as "Oliver"

    When I POST path "/api/vpcs/${vpc_id}/events" with json body expecting a json event stream:
      """
      [
        {
          "kind": "security-group",
          "gt_revision": 0
        },
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
        "type": "tail"
      }
      """

    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "security-group",
        "type": "change",
        "value": ${security_group}
      }
      """

    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "security-group",
        "type": "tail"
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
        "type": "tail"
      }
      """

    # Create another device...
    Given I am logged in as "Oscar"
    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${oscar_user_id}",
        "vpc_id": "${vpc_id}",
        "public_key": "${public_key}",
        "local_ip": "172.17.0.4:58664",
        "tunnel_ip": "",
        "advertise_cidrs": null,
        "relay": false,
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

    # Update the security group
    Given I am logged in as "Oscar"
    When I PATCH path "/api/security-groups/${oscar_security_group_id}" with json body:
      """
      {
        "id": "${oscar_security_group_id}",
        "organization_id": "${oscar_organization_id}",
        "group_description": "update",
        "group_name": "test"
      }
      """
    Then the response code should be 200
    Then the response should match json:
      """
      {
        "id": "${oscar_security_group_id}",
        "organization_id": "${oscar_organization_id}",
        "group_description": "update",
        "group_name": "test",
        "revision": ${response.revision}
      }
      """
    Given I store the ${response} as ${security_group_update}

    # We should get additional change events...
    Given I am logged in as "Oliver"
    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "security-group",
        "type": "change",
        "value": ${security_group_update}
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

