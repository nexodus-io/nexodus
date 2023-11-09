Feature: Web UI

  Background:
    Given a user named "Oscar" with password "testpass"
    Given a user named "EvilBob" with password "testpass"

  Scenario: Bob can login to the UI

    Given I am logged in as "Oscar"
    Then I run playwright script "./tests/login.spec.ts"
