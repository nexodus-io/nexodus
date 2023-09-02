#!/bin/bash

# Quick and dirty way to scrape the error messages from the nexd logs
# from container.
# As we spawn more containers for scale, it helps to keep checking the
# errors in the existing running nexd in case they are getting impacted
# by the scale and throwing errors.

# Flag to print error logs from nexd logs
print_log=false

print_usage() {
    echo "Usage: $0 [-l]"
    echo "  -l    Print error log from nexd running in the container"
    exit 1
}

# Parse command line arguments
while getopts "l" opt; do
    case $opt in
        l) print_log=true ;;
        \?) echo "Invalid option: -$OPTARG" >&2
            print_usage ;;
    esac
done

# Get a list of running container IDs
container_ids=$(docker ps -q)

echo -e "CONTAINER ID\tUSER\t\t\t\t\tERROR COUNT\tJSON\tEOF\tSTUN\tTIMEOUT\t400\t401\t429\t500\t501\t503\tNEW ERROR"
# Loop through each container and execute the command
for container_id in $container_ids; do
    user_id=$(docker inspect --format '{{ index .Config.Labels "user" }}' $container_id)
    logs=$(docker exec "$container_id" sh -c "cat nexodus-logs.txt | grep error" 2>&1)
    error_400=$(echo "$logs" | grep -c "error: 400")
    error_401=$(echo "$logs" | grep -c "error: 401")
    error_429=$(echo "$logs" | grep -c "error: 429")
    error_500=$(echo "$logs" | grep -c "error: 500")
    error_501=$(echo "$logs" | grep -c "error: 501")
    error_503=$(echo "$logs" | grep -c "error: 503")
    error_json=$(echo "$logs" | grep -c "error: json")
    error_eof=$(echo "$logs" | grep -c "EOF")
    error_stun=$(echo "$logs" | grep -ci "STUN")
    error_timeout=$(echo "$logs" | grep -c "timeout")
    known_error=$((error_400 + error_401 + error_429 + error_500 + error_501 + error_503 + error_json + error_eof + error_stun + error_timeout))
    error_count=$(echo "$logs" | grep -v '^$'| wc -l)

    if [ "$error_count" -gt 0 ]; then
	    if [ "$error_count" -eq "$known_error" ]; then
		    echo -e "$container_id\t$user_id\t\t$error_count\t $error_json\t $error_eof\t $error_stun\t $error_timeout\t $error_400\t $error_401\t $error_429\t $error_500\t $error_501\t $error_503\t \e[32mNo\e[0m"
	    else
		echo -e "$container_id\t$user_id\t\t$error_count\t $error_json\t $error_eof\t $error_stun\t $error_timeout\t $error_400\t $error_401\t $error_429\t $error_500\t $error_501\t $error_503\t \e[91mYes\e[0m"
	    fi

	    if [ "$print_log" = true ]; then
	    	echo "$logs"
	    fi
    fi
done
