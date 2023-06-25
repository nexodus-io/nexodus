Feature: Device Metadata API

  Background:
    Given a user named "Bob" with password "testpass"
    Given a user named "Alice" with password "testpass"
    Given a user named "Oliver" with password "testpass"
    Given a user named "Oscar" with password "testpass"

  Scenario: Basic device CRUD operations.

    Given I am logged in as "Bob"

    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${user_id}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${organization_id}

    # Bob creates a device
    Given I generate a new public key as ${public_key}
    When I POST path "/api/devices" with json body:
      """
      {
        "user_id": "${user_id}",
        "organization_id": "${organization_id}",
        "public_key": "${public_key}",
        "endpoints": [{
          "source": "local",
          "address": "172.17.0.3:58664",
          "distance": 0
        }, {
          "source": "stun:stun1.l.google.com:19302",
          "address": "172.17.0.3:58664",
          "distance": 0
        }],
        "tunnel_ip": "",
        "tunnel_ip_v6": "",
        "child_prefix": null,
        "relay": false,
        "discovery": false,
        "endpoint_local_address_ip4": "172.17.0.3",
        "symmetric_nat": true,
        "hostname": "bbac3081d5e8",
        "os": "linux"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${device_id}

    # Bob sets device metadata
    When I PUT path "/api/devices/${device_id}/metadata/tcp:1024" with json body:
      """
      {
        "address": "backend:8080"
      }
      """
    Then the response code should be 200
    And the response should match json:
      """
      {
        "device_id": "${device_id}",
        "key": "tcp:1024",
        "revision": ${response.revision},
        "value": {
          "address": "backend:8080"
        }
      }
      """

    When I PUT path "/api/devices/${device_id}/metadata/udp:1024" with json body:
      """
      {
        "address": "asterisk:5060"
      }
      """
    Then the response code should be 200

    # Update the metadata to let peers know what TLS cert the service uses.
    When I PUT path "/api/devices/${device_id}/metadata/tcp:1024" with json body:
      """
      {
        "address": "backend:8080",
        "cert": "-----BEGIN CERTIFICATE-----\nMIIBgTCCASagAwIBAgIQCe6Y8MgtMMvxr8G0HwOBTzAKBggqhkjOPQQDAjAgMR4w\nHAYDVQQDExVuZXhvZHVzLXNlbGZzaWduZWQtY2EwHhcNMjMwNjAyMTIzMTQxWhcN\ntmfoCOo=\n-----END CERTIFICATE-----"
      }
      """
    Then the response code should be 200
    And the response should match json:
      """
      {
        "device_id": "${device_id}",
        "key": "tcp:1024",
        "revision": ${response.revision},
        "value": {
          "address": "backend:8080",
          "cert": "-----BEGIN CERTIFICATE-----\nMIIBgTCCASagAwIBAgIQCe6Y8MgtMMvxr8G0HwOBTzAKBggqhkjOPQQDAjAgMR4w\nHAYDVQQDExVuZXhvZHVzLXNlbGZzaWduZWQtY2EwHhcNMjMwNjAyMTIzMTQxWhcN\ntmfoCOo=\n-----END CERTIFICATE-----"
        }
      }
      """

    # Bob can get the metadata entry back
    When I GET path "/api/devices/${device_id}/metadata/tcp:1024"
    Then the response code should be 200
    And the response should match json:
      """
      {
        "device_id": "${device_id}",
        "key": "tcp:1024",
        "revision": ${response.revision},
        "value": {
          "address": "backend:8080",
          "cert": "-----BEGIN CERTIFICATE-----\nMIIBgTCCASagAwIBAgIQCe6Y8MgtMMvxr8G0HwOBTzAKBggqhkjOPQQDAjAgMR4w\nHAYDVQQDExVuZXhvZHVzLXNlbGZzaWduZWQtY2EwHhcNMjMwNjAyMTIzMTQxWhcN\ntmfoCOo=\n-----END CERTIFICATE-----"
        }
      }
      """

    # Bob can get all the metadata entries
    When I GET path "/api/devices/${device_id}/metadata"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "device_id": "${device_id}",
          "key": "tcp:1024",
          "revision": ${response[0].revision},
          "value": {
            "address": "backend:8080",
            "cert": "-----BEGIN CERTIFICATE-----\nMIIBgTCCASagAwIBAgIQCe6Y8MgtMMvxr8G0HwOBTzAKBggqhkjOPQQDAjAgMR4w\nHAYDVQQDExVuZXhvZHVzLXNlbGZzaWduZWQtY2EwHhcNMjMwNjAyMTIzMTQxWhcN\ntmfoCOo=\n-----END CERTIFICATE-----"
          }
        },
        {
          "device_id": "${device_id}",
          "key": "udp:1024",
          "revision": ${response[1].revision},
          "value": {
            "address": "asterisk:5060"
          }
        }
      ]
      """

    # We can filter down the keys using the prefix query arg
    When I GET path "/api/devices/${device_id}/metadata?prefix=udp:&prefix=bad:"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "device_id": "${device_id}",
          "key": "udp:1024",
          "revision": ${response[0].revision},
          "value": {
            "address": "asterisk:5060"
          }
        }
      ]
      """

    #
    # Verify Alice can't see Bob's stuff
    #
    Given I am logged in as "Alice"

    # Alice gets an empty list of devices..
    When I GET path "/api/devices/${device_id}/metadata"
    Then the response code should be 404

    When I GET path "/api/devices/${device_id}/metadata/tcp:1024"
    Then the response code should be 404

    When I PATCH path "/api/devices/${device_id}/metadata/tcp:1024" with json body:
      """
      {
        "evil": "pill"
      }
      """
    Then the response code should be 404

    When I DELETE path "/api/devices/${device_id}/metadata/tcp:1024"
    Then the response code should be 404

    When I DELETE path "/api/devices/${device_id}/metadata"
    Then the response code should be 404

    #
    # Switch back to Bob, and make sure he can delete his device.
    #
    Given I am logged in as "Bob"
    When I DELETE path "/api/devices/${device_id}/metadata/tcp:1024"
    Then the response code should be 204

    When I GET path "/api/devices/${device_id}/metadata"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "device_id": "${device_id}",
          "key": "udp:1024",
          "revision": ${response[0].revision},
          "value": {
            "address": "asterisk:5060"
          }
        }
      ]
      """

    When I DELETE path "/api/devices/${device_id}/metadata"
    Then the response code should be 204

    When I GET path "/api/devices/${device_id}/metadata"
    Then the response code should be 200
    And the response should match json:
      """
      []
      """

  Scenario: Using the watch option to stream metadata change events

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
    Given I store the ".id" selection from the response as ${device_id}

    When I PUT path "/api/devices/${device_id}/metadata/tcp:1024" with json body:
      """
      {
        "address": "backend:8080"
      }
      """

    Given I am logged in as "Oliver"
    When I GET path "/api/organizations/${organization_id}/metadata?watch=true&gt_revision=0" as a json event stream
    Then the response code should be 200
    And the response header "Content-Type" should match "application/json;stream=watch"

    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "type": "change",
        "value": {
          "device_id": "${device_id}",
          "key": "tcp:1024",
          "revision": ${response.value.revision},
          "value": {
            "address": "backend:8080"
          }
        }
      }
      """

    # The book mark event signals that you have now received the full list of devices.
    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      { "type": "bookmark" }
      """

    # Create another metadata entry...
    Given I am logged in as "Oscar"
    When I PUT path "/api/devices/${device_id}/metadata/udp:1024" with json body:
      """
      {
        "address": "asterisk:5060"
      }
      """
    Then the response code should be 200

    # We should get additional change events...
    Given I am logged in as "Oliver"
    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "type": "change",
        "value": {
          "device_id": "${device_id}",
          "key": "udp:1024",
          "revision": ${response.value.revision},
          "value": {
            "address": "asterisk:5060"
          }
        }
      }
      """

    Given I am logged in as "Oscar"
    When I DELETE path "/api/devices/${device_id}/metadata/udp:1024"
    Then the response code should be 204

    Given I am logged in as "Oliver"
    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "type": "delete",
        "value": {
          "device_id": "${device_id}",
          "key": "udp:1024",
          "revision": ${response.value.revision},
          "value": {
            "address": "asterisk:5060"
          }
        }
      }
      """
