Feature: Private APIs
  Background:
    Given I port forward to kube resource "svc/apiserver" on port 8080 via local port ${apiserver_port}

  # this scenario cannot be run in parallel with other tests because
  # doing a GC with retention 0s will break event streams which use the
  # soft delte records to discover records that have been recently deleted.
  Scenario: GC soft deleted records

    Given a user named "Oscar" with password "testpass"
    Given I am logged in as "Oscar"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${oscar_user_id}

    # create an additional VPC
    When I POST path "/api/vpcs" with json body:
      """
      {
        "organization_id": "${oscar_user_id}",
        "description": "extra vpc"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${extra_vpc_id}

    # the DB should have two records now
    When I run SQL "SELECT COUNT(*) FROM vpcs WHERE organization_id='${oscar_user_id}'" gives results:
      | count |
      | 2     |

    When I DELETE path "/api/vpcs/${extra_vpc_id}"
    Then the response code should be 200

    # the DB will still have the vpc records, 1 of them will be soft deleted.
    When I run SQL "SELECT COUNT(*) FROM vpcs WHERE organization_id='${oscar_user_id}'" gives results:
      | count |
      | 2     |

    # GC the soft deletes
    When I GET path "http://127.0.0.1:${apiserver_port}/private/gc?retention=0ms"
    Then the response code should be 204

    # Now the soft deleted record should be gone (only the default vpc is left).
    When I run SQL "SELECT COUNT(*) FROM vpcs WHERE organization_id='${oscar_user_id}'" gives results:
      | count |
      | 1     |

  Scenario: Health endpoints

    When I GET path "http://127.0.0.1:${apiserver_port}/private/ready"
    Then the response code should be 200

    When I GET path "http://127.0.0.1:${apiserver_port}/private/live"
    Then the response code should be 200
