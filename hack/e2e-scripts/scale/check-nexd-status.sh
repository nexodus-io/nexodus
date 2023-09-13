#!/bin/bash

# Script picks the containers in each org/user and
# check the status of the nexd agent. By default it
# prints only the agent that is not running.

# Array to store encountered label values
encountered_labels=()

# Get a list of all container IDs that is currently running.
container_ids=$(docker ps -q)

# Print status of all the containers
print_all=false

print_usage() {
	echo "Usage: $0 [-v]"a
	echo "  -v    Print nexd status for all the containers"
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
total_nexd_failed=0
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
		nexds=$(docker ps --filter "label=user=$label_value" --format "{{.ID}}")

		nexd_count=$(echo $nexds | wc -w)
		not_running=0
		# loop through each container in the org
		for nexd in $nexds; do
			# check if the nexd agent is running
			if [ "$(docker exec $nexd nexctl nexd status | grep "Running" | wc -l)" -eq 0 ]; then
				echo -e "container-id: $nexd    user:$label_value    \e[91m Not Running\e[0m"
				not_running=$((not_running + 1))
			else
				if [ "$print_all" = true ]; then
					echo -e "container-id: $nexd    user:$label_value    \e[32m Running\e[0m"
				fi
			fi
		done
		total_nexd_failed=$((total_nexd_failed + not_running))
		echo -e "\e[92m org: $label_value       total-nexd:$nexd_count      Running: $((nexd_count - not_running))     Failed: $not_running\e[0m"
	fi
done

echo -e "\e[92m total-orgs: $total_orgs       total-nexd:$total_nexd       total-nexd-failed: $total_nexd_failed \e[0m"
