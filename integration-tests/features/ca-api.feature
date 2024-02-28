Feature: CA API

  Background:
    Given a user named "Bob" with password "testpass"

  Scenario: Sign a CSR

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
    And the response should match json:
      """
      {
        "description": "my service network",
        "id": "${service_network.id}",
        "organization_id": "${user_id}",
        "revision": ${service_network.revision}
      }
      """

    # Bob asks for his cert to be signed.
    Given I generate a new CSR and key as ${csr_pem}/${cert_key} using:
        """
        Subject:
            CommonName:         "test"
            Country:            ["US"]
            Organization:       ["Example Inc."]
        """

    # He has to be logged in with a device token so that the cert can be associated with the device
    When I POST path "/api/ca/sign" with json body:
      """
      {
        "request": "${csr_pem | json_escape}",
        "usages": ["signing", "key encipherment", "server auth", "client auth"]
      }
      """
    Then the response code should be 403
    And the response should match json:
      """
      {"error":"a device token is required"}
      """

    # Create a site...
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
    Given I store the ".bearer_token" selection from the response as ${site_bearer_token}
    And I decrypt the sealed "${site_bearer_token}" with "${private_key}" and store the result as ${site_bearer_token}

    # Try to sign the CSR again with the site bearer token
    When I set the "Authorization" header to "Bearer ${site_bearer_token}"
    And I POST path "/api/ca/sign" with json body:
      """
      {
        "request": "${csr_pem | json_escape}",
        "usages": ["signing", "key encipherment", "server auth", "client auth"]
      }
      """

    Then the response code should be 200
    Given I store the ${response.certificate | parse_x509_cert} as ${cert}
    Given I store the ${response.ca | parse_x509_cert} as ${ca}

    Then "${cert.Subject.Country.0}" should match "US"
    Then "${cert.Subject.Organization.0}" should match "Example Inc."
    Then "${cert.Subject.CommonName}" should match "test"

    # the CA will set the URIs of cert to identify which site created the cert
    Then "${cert.URIs.0 | string}" should match "spiffe://api.try.nexodus.127.0.0.1.nip.io/o/${user_id}/n/${service_network.id}/s/${site_id}"

    # the CA cert is per Service Network..
    Then "${ca.URIs.0 | string}" should match "spiffe://api.try.nexodus.127.0.0.1.nip.io/o/${user_id}/n/${service_network.id}"
