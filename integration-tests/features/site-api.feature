Feature: Site API

  Background:
    Given a user named "Bob" with password "testpass"
    Given a user named "EvilAlice" with password "testpass"
    Given a user named "Oliver" with password "testpass"
    Given a user named "Oscar" with password "testpass"

  Scenario: Basic site CRUD operations.

    Given I am logged in as "Bob"

    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${user_id}

    # Lets create a Service Network
    When I POST path "/api/service-networks" with json body:
      """
      {
        "organization_id": "${user_id}",
        "description": "my service network"
      }
      """
    Then the response code should be 201
    Given I store the ${response} as ${service_network}

    # Initial site listing should be empty.
    When I GET path "/api/sites"
    Then the response code should be 200
    And the response should match json:
      """
      []
      """

    # Bob creates a site
    Given I generate a new key pair as ${private_key}/${public_key}
    When I POST path "/api/sites" with json body:
      """
      {
        "owner_id": "${user_id}",
        "service_network_id": "${service_network.id}",
        "public_key": "${public_key}",
        "name": "site-a",
        "platform": "kubernetes"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${site_id}
    And the response should match json:
      """
      {
        "id": "${site_id}",
        "revision": ${response.revision},
        "hostname": "",
        "os": "",
        "owner_id": "${user_id}",
        "service_network_id": "${service_network.id}",
        "public_key": "${public_key}",
        "name": "site-a",
        "link_secret": "",
        "bearer_token": "${response.bearer_token}",
        "online": false,
        "online_at": null,
        "platform": "kubernetes"
      }
      """

    # Bob can update his site.
    When I PATCH path "/api/sites/${site_id}" with json body:
      """
      {
        "hostname": "kittenhome"
      }
      """
    Then the response code should be 200
    Given I store the ${response} as ${site1}
    And the response should match json:
      """
      {
        "id": "${site_id}",
        "revision": ${response.revision},
        "hostname": "kittenhome",
        "os": "",
        "owner_id": "${user_id}",
        "service_network_id": "${service_network.id}",
        "public_key": "${public_key}",
        "name": "site-a",
        "link_secret": "",
        "bearer_token": "${response.bearer_token}",
        "online": false,
        "online_at": null,
        "platform": "kubernetes"
      }
      """

    # Bob gets an should see 1 site in the site listing..
    When I GET path "/api/sites"
    Then the response code should be 200
    And the response should match json:
      """
      [{
        "id": "${site_id}",
        "revision": ${response[0].revision},
        "hostname": "kittenhome",
        "os": "",
        "owner_id": "${user_id}",
        "service_network_id": "${service_network.id}",
        "public_key": "${public_key}",
        "name": "site-a",
        "link_secret": "",
        "bearer_token": "${response[0].bearer_token}",
        "online": false,
        "online_at": null,
        "platform": "kubernetes"
      }]
      """

    #
    # Verify EvilAlice can't see Bob's stuff
    #
    Given I am logged in as "EvilAlice"

    # EvilAlice gets an empty list of sites..
    When I GET path "/api/sites"
    Then the response code should be 200
    And the response should match json:
      """
      []
      """

    When I GET path "/api/sites/${site_id}"
    Then the response code should be 404

    When I PATCH path "/api/sites/${site_id}" with json body:
      """
      {
        "hostname": "evilkitten"
      }
      """
    Then the response code should be 404

    When I DELETE path "/api/sites/${site_id}"
    Then the response code should be 404
    And the response should match json:
      """
      {
        "error": "not found",
        "resource": "site"
      }
      """

    #
    # Switch back to Bob, and make sure he can delete his site.
    #
    Given I am logged in as "Bob"
    When I DELETE path "/api/sites/${site_id}"
    Then the response code should be 200
    And the response should match json:
      """
      {
        "id": "${site_id}",
        "revision": ${response.revision},
        "hostname": "kittenhome",
        "os": "",
        "owner_id": "${user_id}",
        "service_network_id": "${service_network.id}",
        "public_key": "",
        "name": "site-a",
        "link_secret": "",
        "online": false,
        "online_at": null,
        "platform": "kubernetes"
      }
      """

    # We should be able to create a new site with the same public key again.
    When I POST path "/api/sites" with json body:
      """
      {
        "owner_id": "${user_id}",
        "service_network_id": "${service_network.id}",
        "public_key": "${public_key}",
        "name": "site-a",
        "platform": "kubernetes"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${site_id}
    And the response should match json:
      """
      {
        "id": "${site_id}",
        "revision": ${response.revision},
        "hostname": "",
        "os": "",
        "owner_id": "${user_id}",
        "service_network_id": "${service_network.id}",
        "public_key": "${public_key}",
        "name": "site-a",
        "link_secret": "",
        "bearer_token": "${response.bearer_token}",
        "online": false,
        "online_at": null,
        "platform": "kubernetes"
      }
      """
    Given I store the ".bearer_token" selection from the response as ${site_bearer_token}
    And I decrypt the sealed "${site_bearer_token}" with "${private_key}" and store the result as ${site_bearer_token}
    And I set the "Authorization" header to "Bearer ${site_bearer_token}"


  Scenario: Using the events endpoint to stream site change events

    Given I am logged in as "Oliver"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oliver_user_id}

    Given I am logged in as "Oscar"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oscar_user_id}

    # Lets create a Service Network
    When I POST path "/api/service-networks" with json body:
      """
      {
        "organization_id": "${oscar_user_id}",
        "description": "my service network"
      }
      """
    Then the response code should be 201
    Given I store the ${response} as ${service_network}

    When I POST path "/api/invitations" with json body:
      """
      {
        "user_id": "${oliver_user_id}",
        "organization_id": "${oscar_user_id}"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${invitation_id}

    Given I am logged in as "Oliver"
    When I POST path "/api/invitations/${invitation_id}/accept"
    Then the response code should be 204
    And the response should match ""

    # Subscribe to the event stream
    Given I am logged in as "Oliver"
    When I POST path "/api/events" with json body expecting a json event stream:
      """
      [
        {
          "kind": "site",
          "gt_revision": 0,
          "options": {
            "service-network-id": "${service_network.id}"
          }
        }
      ]
      """
    Then the response code should be 200
    And the response header "Content-Type" should match "application/json;stream=watch"

    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "site",
        "type": "tail"
      }
      """

    # Create a site...
    Given I am logged in as "Oscar"
    Given I generate a new public key as ${public_key}
    When I POST path "/api/sites" with json body:
      """
      {
        "owner_id": "${oscar_user_id}",
        "service_network_id": "${service_network.id}",
        "public_key": "${public_key}",
        "name": "site-a",
        "platform": "kubernetes"
      }
      """
    Then the response code should be 201
    Given I store the ${response.id} as ${site_id}

    # We should get additional change events...
    Given I am logged in as "Oliver"
    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "site",
        "type": "change",
        "value": {
           "hostname": "",
           "id": "${site_id}",
           "link_secret": "",
           "name": "site-a",
           "os": "",
           "owner_id": "${oscar_user_id}",
           "platform": "kubernetes",
           "public_key": "${public_key}",
           "revision": ${response.value.revision},
           "online": false,
           "online_at": null,
           "service_network_id": "${service_network.id}"
         }
      }
      """

    # Update the security group
    Given I am logged in as "Oscar"
    When I PATCH path "/api/sites/${site_id}" with json body:
      """
      {
        "hostname": "test"
      }
      """
    Then the response code should be 200
    Given I store the ${response} as ${site}

    # We should get additional change events...
    Given I am logged in as "Oliver"
    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "site",
        "type": "change",
        "value": {
           "hostname": "test",
           "id": "${site_id}",
           "link_secret": "",
           "name": "site-a",
           "os": "",
           "owner_id": "${oscar_user_id}",
           "platform": "kubernetes",
           "public_key": "${public_key}",
           "revision": ${response.value.revision},
           "online": false,
           "online_at": null,
           "service_network_id": "${service_network.id}"
         }
      }
      """

    Given I am logged in as "Oscar"
    When I DELETE path "/api/sites/${site_id}"
    Then the response code should be 200
    Given I store the ${response} as ${site}

    Given I am logged in as "Oliver"
    Given I wait up to "3" seconds for a response event
    Then the response should match json:
      """
      {
        "kind": "site",
        "type": "delete",
        "value": {
           "hostname": "test",
           "id": "${site_id}",
           "link_secret": "",
           "name": "site-a",
           "os": "",
           "owner_id": "${oscar_user_id}",
           "platform": "kubernetes",
           "public_key": "",
           "revision": ${response.value.revision},
           "online": false,
           "online_at": null,
           "service_network_id": "${service_network.id}"
         }
      }
      """

