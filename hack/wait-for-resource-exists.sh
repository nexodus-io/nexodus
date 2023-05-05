#!/bin/bash

while ! kubectl get $* > /dev/null 2> /dev/null
do
  echo waiting for $* to exist
  sleep 1
done
