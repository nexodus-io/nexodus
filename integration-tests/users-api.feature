Feature: Users API
  Background:
    Given a user named "Usher" with password "testpass"
    Given a user named "EvilUrsala" with password "testpass"

  Scenario: Basic user CRUD operations.

    # make sure we login and use both users, we are making sure
    # that EvilUrsala can't see Usher
    Given I am logged in as "Usher"
    When I GET path "/api/users/me"
    Then the response code should be 200
    And I store the ".id" selection from the response as ${usher_id}

    Given I am logged in as "EvilUrsala"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${ursala_id}
    And the response should match json:
      """
      {
        "id": "${ursala_id}",
        "username": "${response.username}"
      }
      """

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${organization_id}

    # EvilUrsala can only see her own user id...
    When I GET path "/api/users"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "id": "${ursala_id}",
          "username": "${response[0].username}"
        }
      ]
      """

    # EvilUrsala can get herself by id
    When I GET path "/api/users/${ursala_id}"
    Then the response code should be 200
    And the response should match json:
      """
      {
        "id": "${ursala_id}",
        "username": "${response.username}"
      }
      """

    # EvilUrsala can't get Usher's resource
    When I GET path "/api/users/${usher_id}"
    Then the response code should be 404
    And the response should match json:
      """
      { "error": "not found", "resource": "user" }
      """

    # EvilUrsala can't delete Usher's resource
    When I DELETE path "/api/users/${usher_id}"
    Then the response code should be 404
    And the response should match json:
      """
      { "error": "not found", "resource": "user" }
      """

    # EvilUrsala can delete her own resource
    When I DELETE path "/api/users/${ursala_id}"
    Then the response code should be 200
    And the response should match json:
      """
      {
        "id": "${ursala_id}",
        "username": "${response.username}"
      }
      """
