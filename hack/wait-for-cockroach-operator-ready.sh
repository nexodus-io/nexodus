#!/bin/bash

# Waiting for operator readiness is not enough: see https://github.com/cockroachdb/cockroach-operator/issues/957
while ! kubectl logs -n cockroach-operator-system deploy/cockroach-operator-manager | grep 'serving webhook server' > /dev/null
do
  echo waiting for cockroach-operator-manager to be ready
  sleep 1
done
