Feature: Device API
  Background:
    Given a user named "Bob" with password "testpass"
    Given a user named "Alice" with password "testpass"

  Scenario: Basic device CRUD operations.

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

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${organization_id}


    # Bob gets an empty list of devices..
    When I GET path "/api/devices"
    Then the response code should be 200
    And the response should match json:
      """
      []
      """

    # Bob creates a device
    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${user_id}",
        "organization_id": "${organization_id}",
        "public_key": "${public_key}",
        "local_ip": "172.17.0.3:58664",
        "local_ip_v6": "",
        "tunnel_ip": "",
        "tunnel_ip_v6": "",
        "child_prefix": null,
        "relay": false,
        "discovery": false,
        "reflexive_ip4": "47.196.141.165",
        "endpoint_local_address_ip4": "172.17.0.3",
        "symmetric_nat": true,
        "hostname": "bbac3081d5e8",
        "os": "linux"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${device_id}
    And the response should match json:
      """
      {
        "allowed_ips": [
          "${response.allowed_ips[0]}",
          "${response.allowed_ips[1]}"
        ],
        "child_prefix": null,
        "discovery": false,
        "endpoint_local_address_ip4": "172.17.0.3",
        "hostname": "bbac3081d5e8",
        "id": "${device_id}",
        "local_ip": "172.17.0.3:58664",
        "local_ip_v6": "",
        "organization_id": "${organization_id}",
        "organization_prefix":"${response.organization_prefix}",
        "organization_prefix_v6":"${response.organization_prefix_v6}",
        "os": "linux",
        "public_key": "${public_key}",
        "reflexive_ip4": "47.196.141.165",
        "relay": false,
        "symmetric_nat": true,
        "tunnel_ip": "${response.tunnel_ip}",
        "tunnel_ip_v6": "${response.tunnel_ip_v6}",
        "user_id": "${user_id}"
      }
      """

    # Bob can update his device.
    When I PATCH path "/api/devices/${device_id}" with json body:
      """
      {
        "hostname": "kittenhome"
      }
      """
    Then the response code should be 200
    And the response should match json:
      """
      {
        "allowed_ips": [
          "${response.allowed_ips[0]}",
          "${response.allowed_ips[1]}"
        ],
        "child_prefix": null,
        "discovery": false,
        "endpoint_local_address_ip4": "172.17.0.3",
        "hostname": "kittenhome",
        "id": "${device_id}",
        "local_ip": "172.17.0.3:58664",
        "local_ip_v6": "",
        "organization_id": "${organization_id}",
        "organization_prefix":"${response.organization_prefix}",
        "organization_prefix_v6":"${response.organization_prefix_v6}",
        "os": "linux",
        "public_key": "${public_key}",
        "reflexive_ip4": "47.196.141.165",
        "relay": false,
        "symmetric_nat": true,
        "tunnel_ip": "${response.tunnel_ip}",
        "tunnel_ip_v6": "${response.tunnel_ip_v6}",
        "user_id": "${user_id}"
      }
      """

    # Bob gets an should see 1 device in the device listing..
    When I GET path "/api/devices"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "allowed_ips": [
            "${response[0].allowed_ips[0]}",
            "${response[0].allowed_ips[1]}"
          ],
          "child_prefix": null,
          "discovery": false,
          "endpoint_local_address_ip4": "172.17.0.3",
          "hostname": "kittenhome",
          "id": "${device_id}",
          "local_ip": "172.17.0.3:58664",
          "local_ip_v6": "",
          "organization_id": "${organization_id}",
          "organization_prefix":"${response[0].organization_prefix}",
          "organization_prefix_v6":"${response[0].organization_prefix_v6}",
          "os": "linux",
          "public_key": "${public_key}",
          "reflexive_ip4": "47.196.141.165",
          "relay": false,
          "symmetric_nat": true,
          "tunnel_ip": "${response[0].tunnel_ip}",
          "tunnel_ip_v6": "${response[0].tunnel_ip_v6}",
          "user_id": "${user_id}"
        }
      ]
      """

    #
    # Verify Alice can't see Bob's stuff
    #
    Given I am logged in as "Alice"

    # Alice gets an empty list of devices..
    When I GET path "/api/devices"
    Then the response code should be 200
    And the response should match json:
      """
      []
      """

    When I GET path "/api/devices/${device_id}"
    Then the response code should be 404

    When I PATCH path "/api/devices/${device_id}" with json body:
      """
      {
        "hostname": "evilkitten"
      }
      """
    Then the response code should be 404

    When I DELETE path "/api/devices/${device_id}"
    Then the response code should be 404
    And the response should match json:
      """
      {
        "error": "not found",
        "resource": "device"
      }
      """

    #
    # Switch back to Bob, and make sure he can delete his device.
    #
    Given I am logged in as "Bob"
    When I DELETE path "/api/devices/${device_id}"
    Then the response code should be 200
    And the response should match json:
      """
      {
        "allowed_ips": [
          "${response.allowed_ips[0]}",
          "${response.allowed_ips[1]}"
        ],
        "child_prefix": null,
        "discovery": false,
        "endpoint_local_address_ip4": "172.17.0.3",
        "hostname": "kittenhome",
        "id": "${device_id}",
        "local_ip": "172.17.0.3:58664",
        "local_ip_v6": "",
        "organization_id": "${organization_id}",
        "organization_prefix":"${response.organization_prefix}",
        "organization_prefix_v6":"${response.organization_prefix_v6}",
        "os": "linux",
        "public_key": "${public_key}",
        "reflexive_ip4": "47.196.141.165",
        "relay": false,
        "symmetric_nat": true,
        "tunnel_ip": "${response.tunnel_ip}",
        "tunnel_ip_v6": "${response.tunnel_ip_v6}",
        "user_id": "${user_id}"
      }
      """
