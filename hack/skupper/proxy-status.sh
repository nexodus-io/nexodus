#!/bin/bash

set -e
echo
echo "West proxy status"
echo "======================================================="
echo Device IP: `kubectl --namespace west exec nexodus-router-0 -- /bin/nexctl nexd get tunnelip`
kubectl --namespace west exec nexodus-router-0 -- /bin/nexctl nexd proxy list
echo
echo "East proxy status"
echo "======================================================="
echo Device IP: `kubectl --namespace east exec nexodus-router-0 -- /bin/nexctl nexd get tunnelip`
kubectl --namespace east exec nexodus-router-0 -- /bin/nexctl nexd proxy list