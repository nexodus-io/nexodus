#!/bin/bash

# Script picks the first nexd container in each org/user and
# check the peer connectivity using nexctl nexd peers ping command
# and also dumps the wireguard wg dump with all the tx/rx counters.
# Script is helpful during scale testing where multiple containers
# are spawned across multiple organizations, and it's bit
# time consuming to check if the peers are connected in the
# existing orgs as we spwan new orgs.

# Array to store encountered label values
encountered_labels=()

# Get a list of all container IDs, including stopped ones
container_ids=$(docker ps -q)

# Flag to print wg dump for tx/rx number
print_wg=false

print_usage() {
    echo "Usage: $0 [-d]"
    echo "  -d    Print wg dump information from the container"
    exit 1
}

# Parse command line arguments
while getopts "d" opt; do
    case $opt in
    d) print_wg=true ;;
    \?)
        echo "Invalid option: -$OPTARG" >&2
        print_usage
        ;;
    esac
done

total_nexd=0
total_orgs=0

# Loop through each container
for container_id in $container_ids; do
    total_nexd=$((total_nexd + 1))

    # Get the value of the "user" label for this container
    label_value=$(docker inspect --format '{{ index .Config.Labels "user" }}' $container_id)

    # If the label exists and the value is not in the encountered_labels array
    if [ ! -z "$label_value" ] && [[ ! " ${encountered_labels[@]} " =~ " $label_value " ]]; then
        total_orgs=$((total_orgs + 1))
        # Add the label value to the encountered_labels array
        encountered_labels+=("$label_value")

        peers=$(docker exec -it $container_id wg show wg0 dump | grep -v "off" | wc -l)
        unreachable=$(docker exec $container_id nexctl nexd peers ping | grep "Unreachable" | wc -l)
        # Print the container ID
        if [ $unreachable -gt 0 ]; then
            echo -e "container-id: $container_id    user:$label_value    peers:$peers    \e[91munreachable:$unreachable\e[0m"
        else
            echo -e "container-id: $container_id    user:$label_value    peers:$peers    \e[32munreachable:$unreachable\e[0m"
        fi

        if [ "$print_wg" = true ]; then
            docker exec -it $container_id wg show wg0 dump
        fi
    fi
done

echo -e "\e[92m total-orgs: $total_orgs       total-nexd:$total_nexd\e[0m"
