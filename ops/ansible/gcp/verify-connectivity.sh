#!/bin/sh

trap "echo; exit" INT

# Parse the wireguard addresses from the ansible inventory file
cat $1 | while read output
do
    ping -c 1 -w 2 "$output" > /dev/null
    if [ $? -eq 0 ]; then
    echo "node $output is up"
    else
    echo "node $output is down"
    fi
done