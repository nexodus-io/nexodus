Feature: Site API

  Background:
    Given a user named "Bob" with password "testpass"
    Given a user named "EvilAlice" with password "testpass"

  Scenario: Basic site CRUD operations.

    Given I am logged in as "Bob"

    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${user_id}

    # Initial site listing should be empty.
    When I GET path "/api/sites"
    Then the response code should be 200
    And the response should match json:
      """
      []
      """

    # Bob creates a site
    Given I generate a new public key as ${public_key}
    When I POST path "/api/sites" with json body:
      """
      {
        "owner_id": "${user_id}",
        "vpc_id": "${user_id}",
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
        "vpc_id": "${user_id}",
        "public_key": "${public_key}",
        "name": "site-a",
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
        "vpc_id": "${user_id}",
        "public_key": "${public_key}",
        "name": "site-a",
        "platform": "kubernetes"
      }
      """

    # Bob gets an should see 1 site in the site listing..
    When I GET path "/api/sites"
    Then the response code should be 200
    And the response should match json:
      """
      [
        ${site1}
      ]
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
        "vpc_id": "${user_id}",
        "public_key": "",
        "name": "site-a",
        "platform": "kubernetes"
      }
      """

    # We should be able to create a new site with the same public key again.
    When I POST path "/api/sites" with json body:
      """
      {
        "owner_id": "${user_id}",
        "vpc_id": "${user_id}",
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
        "vpc_id": "${user_id}",
        "public_key": "${public_key}",
        "name": "site-a",
        "platform": "kubernetes"
      }
      """