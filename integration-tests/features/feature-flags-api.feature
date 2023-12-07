Feature: Feature Flags API
  Background:
    Given a user named "greg" with password "testpass"

  Scenario: List the feature flags as Greg

    Given I am logged in as "greg"
    When I GET path "/api/fflags"
    Then the response code should be 200
    And the response should match json:
      """
      {
        "ca": true,
        "multi-organization": true,
        "security-groups": true,
        "devices": true,
        "sites": true
      }
      """

  Scenario: List the feature flags when not logged in

    Given I am not logged in
    When I GET path "/api/fflags"
    Then the response code should be 401
    And the response should match:
      """
      Jwt is missing
      """
