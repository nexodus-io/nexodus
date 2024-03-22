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

    # Each user can create devices in the org:
    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${oliver_user_id}",
        "vpc_id": "${vpc_id}",
        "public_key": "${public_key}",
        "local_ip": "172.17.0.3:58664",
        "tunnel_ip": "",
        "advertise_cidrs": null,
        "relay": false,
        "symmetric_nat": true,
        "hostname": "oliver-laptop"
      }
      """
    Then the response code should be 201
    Given I store the ${response} as ${oliver_device}
    And the response should match json:
      """
      {
        "bearer_token": "${response.bearer_token}",
        "advertise_cidrs": null,
        "allowed_ips":${oliver_device.allowed_ips} ,
        "endpoints": null,
        "hostname": "oliver-laptop",
        "id": "${oliver_device.id}",
        "ipv4_tunnel_ips": ${oliver_device.ipv4_tunnel_ips},
        "ipv6_tunnel_ips": ${oliver_device.ipv6_tunnel_ips},
        "online": false,
        "online_at": null,
        "os": "",
        "owner_id": "${oliver_user_id}",
        "public_key": "${public_key}",
        "relay": false,
        "revision": ${oliver_device.revision},
        "security_group_id": "${oliver_device.security_group_id}",
        "symmetric_nat": true,
        "vpc_id": "${vpc_id}"
      }
      """

    Given I am logged in as "Oscar"
    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${oscar_user_id}",
        "vpc_id": "${vpc_id}",
        "public_key": "${public_key}",
        "local_ip": "172.17.0.1:58664",
        "tunnel_ip": "",
        "advertise_cidrs": null,
        "relay": false,
        "symmetric_nat": true,
        "hostname": "oscar-pc"
      }
      """
    Then the response code should be 201
    Given I store the ${response} as ${oscar_device}
    And the response should match json:
      """
      {
        "bearer_token": "${response.bearer_token}",
        "advertise_cidrs": null,
        "allowed_ips":${oscar_device.allowed_ips} ,
        "endpoints": null,
        "hostname": "oscar-pc",
        "id": "${oscar_device.id}",
        "ipv4_tunnel_ips": ${oscar_device.ipv4_tunnel_ips},
        "ipv6_tunnel_ips": ${oscar_device.ipv6_tunnel_ips},
        "online": false,
        "online_at": null,
        "os": "",
        "owner_id": "${oscar_user_id}",
        "public_key": "${public_key}",
        "relay": false,
        "revision": ${oscar_device.revision},
        "security_group_id": "${oscar_device.security_group_id}",
        "symmetric_nat": true,
        "vpc_id": "${vpc_id}"
      }
      """

    # Users that are not in the org should not be able to add devices to the org.
    Given I am logged in as "EvilBob"
    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${evilbob_user_id}",
        "vpc_id": "${vpc_id}",
        "public_key": "${public_key}",
        "local_ip": "172.17.0.21:58664",
        "tunnel_ip": "",
        "advertise_cidrs": null,
        "relay": false,
        "symmetric_nat": true,
        "hostname": "bob-pc"
      }
      """
    Then the response code should be 404
    And the response should match json:
      """
      { "error": "not found", "resource": "vpc" }
      """

    # They both can see the devices
    Given I am logged in as "Oscar"
    When I GET path "/api/vpcs/${vpc_id}/devices"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "advertise_cidrs": null,
          "allowed_ips":${oliver_device.allowed_ips} ,
          "endpoints": null,
          "hostname": "oliver-laptop",
          "id": "${oliver_device.id}",
          "ipv4_tunnel_ips": ${oliver_device.ipv4_tunnel_ips},
          "ipv6_tunnel_ips": ${oliver_device.ipv6_tunnel_ips},
          "online": false,
          "online_at": null,
          "os": "",
          "owner_id": "${oliver_user_id}",
          "public_key": "${oliver_device.public_key}",
          "relay": false,
          "revision": ${oliver_device.revision},
          "security_group_id": "${oliver_device.security_group_id}",
          "symmetric_nat": true,
          "vpc_id": "${vpc_id}"
        },
        {
          "bearer_token": "${response[1].bearer_token}",
          "advertise_cidrs": null,
          "allowed_ips":${oscar_device.allowed_ips} ,
          "endpoints": null,
          "hostname": "oscar-pc",
          "id": "${oscar_device.id}",
          "ipv4_tunnel_ips": ${oscar_device.ipv4_tunnel_ips},
          "ipv6_tunnel_ips": ${oscar_device.ipv6_tunnel_ips},
          "online": false,
          "online_at": null,
          "os": "",
          "owner_id": "${oscar_user_id}",
          "public_key": "${oscar_device.public_key}",
          "relay": false,
          "revision": ${oscar_device.revision},
          "security_group_id": "${oscar_device.security_group_id}",
          "symmetric_nat": true,
          "vpc_id": "${vpc_id}"
        }
      ]
      """

    Given I am logged in as "Oliver"
    When I GET path "/api/vpcs/${vpc_id}/devices"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "bearer_token": "${response[0].bearer_token}",
          "advertise_cidrs": null,
          "allowed_ips":${oliver_device.allowed_ips} ,
          "endpoints": null,
          "hostname": "oliver-laptop",
          "id": "${oliver_device.id}",
          "ipv4_tunnel_ips": ${oliver_device.ipv4_tunnel_ips},
          "ipv6_tunnel_ips": ${oliver_device.ipv6_tunnel_ips},
          "online": false,
          "online_at": null,
          "os": "",
          "owner_id": "${oliver_user_id}",
          "public_key": "${oliver_device.public_key}",
          "relay": false,
          "revision": ${oliver_device.revision},
          "security_group_id": "${oliver_device.security_group_id}",
          "symmetric_nat": true,
          "vpc_id": "${vpc_id}"
        },
        {
          "advertise_cidrs": null,
          "allowed_ips":${oscar_device.allowed_ips} ,
          "endpoints": null,
          "hostname": "oscar-pc",
          "id": "${oscar_device.id}",
          "ipv4_tunnel_ips": ${oscar_device.ipv4_tunnel_ips},
          "ipv6_tunnel_ips": ${oscar_device.ipv6_tunnel_ips},
          "online": false,
          "online_at": null,
          "os": "",
          "owner_id": "${oscar_user_id}",
          "public_key": "${oscar_device.public_key}",
          "relay": false,
          "revision": ${oscar_device.revision},
          "security_group_id": "${oscar_device.security_group_id}",
          "symmetric_nat": true,
          "vpc_id": "${vpc_id}"
        }
      ]
      """

    # Other user's should not see the devices.
    Given I am logged in as "EvilBob"
    When I GET path "/api/vpcs/${vpc_id}/devices"
    Then the response code should be 404
    And the response should match json:
      """
      { "error": "not found", "resource": "vpc" }
      """
