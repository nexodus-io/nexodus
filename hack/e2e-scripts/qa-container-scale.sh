#!/bin/bash
# Usage:
# qa-container-scale.sh --kc-password "<ADMIN_KEYCLOAK_PASSWORD>" --nexd-password "<PASS_CAN_BE_ANYTHING>" --nexd-count 3
# Connect to the containers after the script is run:
# docker exec -it <CID> bash
# Once on a container, verify connectivity:
# nexctl nexd peers ping
# Cleanup the container individually or delete all running containers:
# docker rm -f $(docker ps -a -q)
# The script assumes the user can run docker by adding the current user to the docker group:
# sudo groupadd docker
# sudo usermod -aG docker $USER

# API Server URLs
NEXODUS_API_SERVER="https://qa.nexodus.io" # example https://try.nexodus.io
NEXODUS_AUTH_SERVER="https://auth.qa.nexodus.io" # example https://auth.try.nexodus.io
NEXD_IMAGE="quay.io/nexodus/nexd" # example quay.io/nexodus/nexd-qa

# Function to check for required tools
check_requirements() {
  for cmd in "docker" "go"
  do
    if ! command -v $cmd &> /dev/null
    then
        echo "$cmd could not be found. Please install it to proceed."
        exit 1
    fi
  done
}

# Function to launch docker containers
launch_containers() {
  for i in $(seq 1 $1)
  do
    container_id=$(docker run -l user=$2 -d --network bridge \
            --cap-add SYS_MODULE \
            --cap-add NET_ADMIN \
            --cap-add NET_RAW \
            --sysctl net.ipv4.ip_forward=1 \
            --sysctl net.ipv6.conf.all.disable_ipv6=0 \
            --sysctl net.ipv6.conf.all.forwarding=1 \
            $NEXD_IMAGE sleep 100000)

    docker cp nexd-init.sh $container_id:/
    docker exec $container_id chmod +x /nexd-init.sh
    docker exec $container_id sh -c "nohup sh ./nexd-init.sh > nexodus-logs.txt 2>&1 &"
    # Uncomment the following to add a pause between container launches for variations in the scale testing
    # sleep 1
  done
}

# Function to print help message
print_help() {
  echo "Usage: $0 --kc-password <password> --nexd-password <password> --nexd-count <count>"
  echo ""
  echo "Arguments:"
  echo "--kc-password <password>: The keycloak password for the kctool command."
  echo "--nexd-password <password>: The user password for the nexd command."
  echo "--nexd-count <count>: The number of nexd containers to launch and attach to the api-server."
  echo "-d |--deployment <deployment>: The deployment to use. Valid values are prod,qa and playground. Default is qa."
  echo "-h |--help: Prints this help message."
  exit 1
}

# Call the function to check requirements
check_requirements

# Default passwords and count
KC_PASSWORD=""
NEXD_PASSWORD=""
NEXD_COUNT=""

# Parse command line arguments
while (( "$#" )); do
  case "$1" in
    --kc-password)
      KC_PASSWORD="$2"
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
    --deployment|-d)
      case "$2" in
        "prod")
           NEXODUS_API_SERVER="https://try.nexodus.io"
           NEXODUS_AUTH_SERVER="https://auth.try.nexodus.io"
           ;;
        "playground")
           NEXODUS_API_SERVER="https://playground.nexodus.io"
           NEXODUS_AUTH_SERVER="https://auth.playground.nexodus.io"
           ;;
        *)
           ;;
      esac
      shift 2
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

# Check if passwords and container count were provided
if [ -z "$KC_PASSWORD" ] || [ -z "$NEXD_PASSWORD" ] || [ -z "$NEXD_COUNT" ]; then
    print_help
fi

# Check if the directory exists
if [ ! -d "kctool" ]; then
  echo "kctool directory was not found. Please ensure you are running the script from the nexodus/hack/e2e-scripts/ directory"
  exit 1
fi

pushd kctool
go build -o kctool
popd

# The keycloak password is passed as an argument
NEXD_USER=$(./kctool/kctool --create-user -ku admin -u qa -kp "$KC_PASSWORD" -p "$NEXD_PASSWORD" $NEXODUS_AUTH_SERVER)

cat << EOF > nexd-init.sh
#!/bin/sh
echo "Running command: nexd  --service-url $NEXODUS_API_SERVER  --username <username> --password <password>" > nexodus-logs.txt
NEXD_LOGLEVEL=debug nexd \
--service-url '$NEXODUS_API_SERVER' \
--username '$NEXD_USER' \
--password '$NEXD_PASSWORD' 2>&1
EOF

# Call the function to launch containers and attach the nodes to the api-server
launch_containers $NEXD_COUNT $NEXD_USER
