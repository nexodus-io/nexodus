#!/bin/sh

# Parse the wireguard addresses from the ansible inventory file
grep wireguard_allowed_ips inventory.txt | awk '{print $5}' | awk -F "=" '{print $2}' | awk -F "/" '{print $1}' | while read output
do
    ping -c 1 "$output" > /dev/null
    if [ $? -eq 0 ]; then
    echo "node $output is up"
    else
    echo "node $output is down"
    fi
done