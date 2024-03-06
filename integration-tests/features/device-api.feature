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
        "full_name": "Test Bob",
        "picture": "",
        "username": "${response.username}"
      }
      """

    When I GET path "/api/vpcs"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${vpc_id}
    And ${vpc_id} is not empty

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].security_group_id} as ${security_group_id}

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
        "advertise_cidrs": null,
        "relay": false,
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
        "online": false,
        "online_at": null,
        "advertise_cidrs": null,
        "endpoints": [{
          "source": "local",
          "address": "172.17.0.3:58664"
        }, {
          "source": "stun:stun1.l.google.com:19302",
          "address": "172.17.0.3:58664"
        }],
        "hostname": "bbac3081d5e8",
        "id": "${device_id}",
        "vpc_id": "${vpc_id}",
        "os": "linux",
        "public_key": "${public_key}",
        "relay": false,
        "revision": ${response.revision},
        "symmetric_nat": true,
        "ipv4_tunnel_ips": [
          {
            "address": "${response.ipv4_tunnel_ips[0].address}",
            "cidr": "${response.ipv4_tunnel_ips[0].cidr}"
          }
        ],
        "ipv6_tunnel_ips": [
          {
            "address": "${response.ipv6_tunnel_ips[0].address}",
            "cidr": "${response.ipv6_tunnel_ips[0].cidr}"
          }
        ],
        "owner_id": "${user_id}",
        "security_group_id": "${response.security_group_id}"
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
        "online": false,
        "online_at": null,
        "advertise_cidrs": null,
        "endpoints": [{
          "source": "local",
          "address": "172.17.0.3:58664"
        }, {
          "source": "stun:stun1.l.google.com:19302",
          "address": "172.17.0.3:58664"
        }],
        "hostname": "kittenhome",
        "id": "${device_id}",
        "vpc_id": "${vpc_id}",
        "os": "linux",
        "public_key": "${public_key}",
        "relay": false,
        "revision": ${response.revision},
        "symmetric_nat": true,
        "ipv4_tunnel_ips": [
          {
            "address": "${response.ipv4_tunnel_ips[0].address}",
            "cidr": "${response.ipv4_tunnel_ips[0].cidr}"
          }
        ],
        "ipv6_tunnel_ips": [
          {
            "address": "${response.ipv6_tunnel_ips[0].address}",
            "cidr": "${response.ipv6_tunnel_ips[0].cidr}"
          }
        ],        "owner_id": "${user_id}",
        "security_group_id": "${response.security_group_id}"
      }
      """

    # Bob can update if device become relay.
    When I PATCH path "/api/devices/${device_id}" with json body:
      """
      {
        "relay": true
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
        "online": false,
        "online_at": null,
        "advertise_cidrs": null,
        "endpoints": [{
          "source": "local",
          "address": "172.17.0.3:58664"
        }, {
          "source": "stun:stun1.l.google.com:19302",
          "address": "172.17.0.3:58664"
        }],
        "hostname": "kittenhome",
        "id": "${device_id}",
        "vpc_id": "${vpc_id}",
        "os": "linux",
        "public_key": "${public_key}",
        "relay": true,
        "revision": ${response.revision},
        "symmetric_nat": true,
        "ipv4_tunnel_ips": [
          {
            "address": "${response.ipv4_tunnel_ips[0].address}",
            "cidr": "${response.ipv4_tunnel_ips[0].cidr}"
          }
        ],
        "ipv6_tunnel_ips": [
          {
            "address": "${response.ipv6_tunnel_ips[0].address}",
            "cidr": "${response.ipv6_tunnel_ips[0].cidr}"
          }
        ],        "owner_id": "${user_id}",
        "security_group_id": "${response.security_group_id}"
      }
      """

    # Bob can update if device is no more behind symmetric NAT.
    When I PATCH path "/api/devices/${device_id}" with json body:
      """
      {
        "symmetric_nat": false
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
        "online": false,
        "online_at": null,
        "advertise_cidrs": null,
        "endpoints": [{
          "source": "local",
          "address": "172.17.0.3:58664"
        }, {
          "source": "stun:stun1.l.google.com:19302",
          "address": "172.17.0.3:58664"
        }],
        "hostname": "kittenhome",
        "id": "${device_id}",
        "vpc_id": "${vpc_id}",
        "os": "linux",
        "public_key": "${public_key}",
        "relay": true,
        "revision": ${response.revision},
        "symmetric_nat": false,
        "ipv4_tunnel_ips": [
          {
            "address": "${response.ipv4_tunnel_ips[0].address}",
            "cidr": "${response.ipv4_tunnel_ips[0].cidr}"
          }
        ],
        "ipv6_tunnel_ips": [
          {
            "address": "${response.ipv6_tunnel_ips[0].address}",
            "cidr": "${response.ipv6_tunnel_ips[0].cidr}"
          }
        ],        "owner_id": "${user_id}",
        "security_group_id": "${response.security_group_id}"
      }
      """

    # Bob gets the devices and should see 1 device in the device listing..
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
          "online": false,
          "online_at": null,
          "advertise_cidrs": null,
          "endpoints": [{
            "source": "local",
            "address": "172.17.0.3:58664"
          }, {
            "source": "stun:stun1.l.google.com:19302",
            "address": "172.17.0.3:58664"
          }],
          "hostname": "kittenhome",
          "id": "${device_id}",
          "vpc_id": "${vpc_id}",
          "os": "linux",
          "public_key": "${public_key}",
          "relay": true,
          "revision": ${response[0].revision},
          "symmetric_nat": false,
          "security_group_id": "${response[0].security_group_id}",
          "ipv4_tunnel_ips": [
            {
              "address": "${response[0].ipv4_tunnel_ips[0].address}",
              "cidr": "${response[0].ipv4_tunnel_ips[0].cidr}"
            }
          ],
          "ipv6_tunnel_ips": [
            {
              "address": "${response[0].ipv6_tunnel_ips[0].address}",
              "cidr": "${response[0].ipv6_tunnel_ips[0].cidr}"
            }
          ],
          "owner_id": "${user_id}"
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
        "online": false,
        "online_at": null,
        "advertise_cidrs": null,
        "endpoints": [{
          "source": "local",
          "address": "172.17.0.3:58664"
        }, {
          "source": "stun:stun1.l.google.com:19302",
          "address": "172.17.0.3:58664"
        }],
        "hostname": "kittenhome",
        "id": "${device_id}",
        "vpc_id": "${vpc_id}",
        "os": "linux",
        "public_key": "",
        "relay": true,
        "revision": ${response.revision},
        "symmetric_nat": false,
        "ipv4_tunnel_ips": [
          {
            "address": "${response.ipv4_tunnel_ips[0].address}",
            "cidr": "${response.ipv4_tunnel_ips[0].cidr}"
          }
        ],
        "ipv6_tunnel_ips": [
          {
            "address": "${response.ipv6_tunnel_ips[0].address}",
            "cidr": "${response.ipv6_tunnel_ips[0].cidr}"
          }
        ],
        "owner_id": "${user_id}",
        "security_group_id": "${response.security_group_id}"
      }
      """

      # We should be able to create a new device with the same public key again.
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
        "advertise_cidrs": null,
        "relay": false,
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
        "online": false,
        "online_at": null,
        "advertise_cidrs": null,
        "endpoints": [{
          "source": "local",
          "address": "172.17.0.3:58664"
        }, {
          "source": "stun:stun1.l.google.com:19302",
          "address": "172.17.0.3:58664"
        }],
        "hostname": "bbac3081d5e8",
        "id": "${device_id}",
        "vpc_id": "${vpc_id}",
        "os": "linux",
        "public_key": "${public_key}",
        "relay": false,
        "revision": ${response.revision},
        "symmetric_nat": true,
        "ipv4_tunnel_ips": [
          {
            "address": "${response.ipv4_tunnel_ips[0].address}",
            "cidr": "${response.ipv4_tunnel_ips[0].cidr}"
          }
        ],
        "ipv6_tunnel_ips": [
          {
            "address": "${response.ipv6_tunnel_ips[0].address}",
            "cidr": "${response.ipv6_tunnel_ips[0].cidr}"
          }
        ],
        "owner_id": "${user_id}",
        "security_group_id": "${response.security_group_id}"
      }
      """