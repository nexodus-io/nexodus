#!/bin/bash
# Usage:
# /qa-kind-container-scale.sh  --nexd-user kitteh1 --nexd-password "floofykittens" --nexd-count 3 --api-server-ip x.x.x.x
# Connect to the containers after the script is run:
# $CONTAINER_RUNTIME exec -it <CID> bash
# Once on a container, verify connectivity:
# nexctl nexd peers ping
# Cleanup the container individually or delete all running containers:
# $CONTAINER_RUNTIME rm -f $($CONTAINER_RUNTIME ps -a -q)
# The script assumes the user can run docker by adding the current user to the docker group:
# sudo groupadd docker
# sudo usermod -aG docker $USER

# Setting the default container runtime to docker
CONTAINER_RUNTIME="docker"

# API Server URLs
CUSTOM_API_SERVER_IP=""
NEXD_IMAGE="quay.io/nexodus/nexd" # example quay.io/nexodus/nexd-qa
CUSTOM_CERT=false

# Function to check for required tools
check_requirements() {
  # Check for docker if it's the selected runtime
  if [ "$CONTAINER_RUNTIME" == "docker" ] && ! command -v docker &> /dev/null
  then
      echo "docker could not be found. Please install it to proceed."
      exit 1
  fi
  # Check for podman if it's the selected runtime
  if [ "$CONTAINER_RUNTIME" == "sudo podman" ] && ! command -v podman &> /dev/null
  then
      echo "podman could not be found. Please install it to proceed."
      exit 1
  fi
}

# Function to launch docker containers
launch_containers() {
  for i in $(seq 1 $1)
  do
    container_id=$($CONTAINER_RUNTIME run -d --network bridge \
            --cap-add SYS_MODULE \
            --cap-add NET_ADMIN \
            --cap-add NET_RAW \
            --sysctl net.ipv4.ip_forward=1 \
            --sysctl net.ipv6.conf.all.disable_ipv6=0 \
            --sysctl net.ipv6.conf.all.forwarding=1 \
            $NEXD_IMAGE sleep 100000)

    # Add the custom cert
    $CONTAINER_RUNTIME exec $container_id mkdir /.certs/
    $CONTAINER_RUNTIME cp rootCA.pem $container_id:/.certs/
    $CONTAINER_RUNTIME exec $container_id chmod 0644 /.certs/rootCA.pem
    $CONTAINER_RUNTIME exec $container_id sh -c "CAROOT=/.certs mkcert -install"

    # Add the custom api-server IP
    line="$CUSTOM_API_SERVER_IP auth.try.nexodus.127.0.0.1.nip.io api.try.nexodus.127.0.0.1.nip.io try.nexodus.127.0.0.1.nip.io"
    echo "$line" | $CONTAINER_RUNTIME exec -i $container_id tee -a /etc/hosts

    $CONTAINER_RUNTIME cp nexd-init.sh $container_id:/
    $CONTAINER_RUNTIME exec $container_id chmod +x /nexd-init.sh
    $CONTAINER_RUNTIME exec $container_id sh -c "nohup sh ./nexd-init.sh > nexodus-logs.txt 2>&1 &"
    # Uncomment the following to add a pause between container launches for variations in the scale testing
    # sleep 1
  done
}

# Function to print help message
print_help() {
  echo "Usage: $0 --kc-password <password> --nexd-password <password> --nexd-count <count> [--custom-script]"
  echo ""
  echo "Arguments:"
  echo "-nexd-user username."
  echo "-nexd-password <password>: The user password for the nexd command."
  echo "--nexd-count <count>: The number of nexd containers to launch and attach to the api-server."
  echo "--custom-cert: Enable custom modifications (modifying hosts and importing certs)."
  echo "--api-server-ip <ip_address>: The IP address of the custom API server."
  echo "-h | --help: Prints this help message."
  exit 1
}

# Default passwords and count
NEXD_PASSWORD=""
NEXD_COUNT=""
NEXD_USER=""

# Parse command line arguments
while (( "$#" )); do
  case "$1" in
    --nexd-user)
      NEXD_USER="$2"
      shift 2
      ;;
    --nexd-password)
      NEXD_PASSWORD="$2"
      shift 2
      ;;
    --nexd-count)
      NEXD_COUNT="$2"
      shift 2
      ;;
    --api-server-ip)
     CUSTOM_API_SERVER_IP="$2"
     shift 2
     ;;
    --podman)
      CONTAINER_RUNTIME="sudo podman"
      shift
      ;;
    -h|--help)
      print_help
      ;;
    *)
      echo "Unknown option $1"
      print_help
      ;;
  esac
done

# Call the function to check requirements
check_requirements

# Check if username, password, api server IP, and container count were provided
if [ -z "$NEXD_USER" ] || [ -z "$NEXD_PASSWORD" ] || [ -z "$NEXD_COUNT" ] || [ -z "$CUSTOM_API_SERVER_IP" ]; then
   print_help
fi

# Check if rootCA.pem exists
if [ ! -f "rootCA.pem" ]; then
  echo "rootCA.pem is required to run the script. Please ensure it's in the same directory as the script."
  exit 1
fi

cat << EOF > nexd-init.sh
#!/bin/sh
echo "Running command: nexd  --service-url https://try.nexodus.127.0.0.1.nip.io  --username <username> --password <password>" > nexodus-logs.txt
NEXD_LOGLEVEL=debug nexd \
--service-url https://try.nexodus.127.0.0.1.nip.io \
--username '$NEXD_USER' \
--password '$NEXD_PASSWORD' 2>&1
EOF

# Call the function to launch containers and attach the nodes to the api-server
launch_containers $NEXD_COUNT
