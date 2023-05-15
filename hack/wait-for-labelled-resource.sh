#!/bin/bash

until test $(kubectl get $* -o json | jq '.items | length') -gt 0
do
  echo waiting for $* to exist
  sleep 1
done
