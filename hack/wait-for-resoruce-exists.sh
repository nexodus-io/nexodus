#!/bin/bash

# Waiting for operator readiness is not enough: see https://github.com/cockroachdb/cockroach-operator/issues/957
while ! kubectl get $* > /dev/null 2> /dev/null
do
  echo waiting for $* to exist
  sleep 1
done
