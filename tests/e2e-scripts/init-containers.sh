#!/bin/bash

set -ex

install_docker() {
    ###########################################################################
    # Description:                                                            #
    # Install Docker                                                          #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    sudo apt-get update
    sudo apt-get install -y \
        ca-certificates \
        curl \
        gnupg \
        lsb-release
    sudo mkdir -p /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    echo \
        "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null

    sudo apt-get update
    sudo apt-get install -y docker-ce docker-ce-cli containerd.io
}

start_containers() {
    ###########################################################################
    # Description:                                                            #
    # Start the redis broker instance, and two Docker edge nodes              #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    # Start redis server
    sudo docker run \
        --name redis \
        -d -p 6379:6379 \
        redis redis-server \
        --requirepass floofykittens

    # Start node1 (container image is generic until the cli stabilizes so no arguments, a script below builds the jaywalk cmd)
    sudo docker run -itd \
        --name=node1 \
        --cap-add=SYS_MODULE \
        --cap-add=NET_ADMIN \
        quay.io/networkstatic/wireguard

    # Start node2
    sudo docker run -itd \
        --name=node2 \
        --cap-add=SYS_MODULE \
        --cap-add=NET_ADMIN \
        quay.io/networkstatic/wireguard
}

start_supervisor() {
    ###########################################################################
    # Description:                                                            #
    # Start the supervisor/controller instance                                #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    local redis_ip=$(sudo docker inspect --format "{{ .NetworkSettings.IPAddress }}" redis)
    ../supervisor/jaywalk-supervisor \
        -streamer-address ${redis_ip} \
        -streamer-passwd floofykittens >supervisor-logs.txt &
}

copy_binaries() {
    ###########################################################################
    # Description:                                                            #
    # Copy the binaries and create the container script to start the agent    #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    # Shared controller address
    local controller=$(sudo docker inspect --format "{{ .NetworkSettings.IPAddress }}" redis)
    local controller_passwd=floofykittens
    local zone=default

    # node1 specific details
    local node1_pubkey=AbZ1fPkCbjYAe9D61normbb7urAzMGaRMDVyR5Bmzz4=
    local node1_pvtkey=8GtvCMlUsFVoadj0B3Y3foy7QbKJB9vcq5R+Mpc7OlE=
    local node1_ip=$(sudo docker inspect --format "{{ .NetworkSettings.IPAddress }}" node1)

    # node2 specific details
    local node2_pubkey=oJlDE1y9xxmR6CIEYCSJAN+8b/RK73TpBYixlFiBJDM=
    local node2_pvtkey=cGXbnP3WKIYbIbEyFpQ+kziNk/kHBM8VJhslEG8Uj1c=
    local node2_ip=$(sudo docker inspect --format "{{ .NetworkSettings.IPAddress }}" node2)

    chmod +x e2e-scripts/create-jaywalk-startup.sh
    # Create jaywalk startup script for node1
    e2e-scripts/create-jaywalk-startup.sh ${node1_pubkey} ${node1_pvtkey} ${controller} ${node1_ip} ${controller_passwd} ${zone} jaywalk-run-node1.sh
    # Create jaywalk startup script for node2
    e2e-scripts/create-jaywalk-startup.sh ${node2_pubkey} ${node2_pvtkey} ${controller} ${node2_ip} ${controller_passwd} ${zone} jaywalk-run-node2.sh

    # STDOUT the scripts for debugging
    cat jaywalk-run-node1.sh
    cat jaywalk-run-node2.sh

    # Set permissions
    chmod +x ../jaywalk-agent/jaywalk
    chmod +x ../supervisor/jaywalk-supervisor

    # Copy binaries and scripts (copying the supervisor even though we are running it on the VM instead of in a container)
    sudo docker cp ../jaywalk-agent/jaywalk node1:/bin/jaywalk
    sudo docker cp ../jaywalk-agent/jaywalk node2:/bin/jaywalk
    sudo docker cp ../supervisor/jaywalk-supervisor node1:/bin/jaywalk-supervisor
    sudo docker cp ../supervisor/jaywalk-supervisor node2:/bin/jaywalk-supervisor
    sudo docker cp ./jaywalk-run-node1.sh node1:/bin/jaywalk-run-node1.sh
    sudo docker cp ./jaywalk-run-node2.sh node2:/bin/jaywalk-run-node2.sh

    # Set permissions in the container
    sudo docker exec node1 chmod +x /bin/jaywalk-run-node1.sh
    sudo docker exec node2 chmod +x /bin/jaywalk-run-node2.sh

    # Start the agents on both nodes
    sudo docker exec node1 /bin/jaywalk-run-node1.sh &
    sudo docker exec node2 /bin/jaywalk-run-node2.sh &
    sleep 1
}

verify_connectivity() {
    ###########################################################################
    # Description:                                                            #
    # Verify the container can reach one another                              #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    # Check connectivty between node1 -> node2
    if sudo docker exec node1 ping -c 2 -w 2 $(sudo docker exec node2 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node1 failed to reach node2, e2e failed"
        exit 1
    fi
    # Check connectivty between node2 -> node1
    if sudo docker exec node2 ping -c 2 -w 2 $(sudo docker exec node1 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node2 failed to reach node1, e2e failed"
        exit 1
    fi
}

setup_custom_zone_connectivity() {
    ###########################################################################
    # Description:                                                            #
    # Verify the container can reach one another                              #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    # Shared controller address
    local controller=$(sudo docker inspect --format "{{ .NetworkSettings.IPAddress }}" redis)
    local controller_passwd=floofykittens
    local zone=zone-blue

    # node1 specific details
    local node1_pubkey=AbZ1fPkCbjYAe9D61normbb7urAzMGaRMDVyR5Bmzz4=
    local node1_pvtkey=8GtvCMlUsFVoadj0B3Y3foy7QbKJB9vcq5R+Mpc7OlE=
    local node1_ip=$(sudo docker inspect --format "{{ .NetworkSettings.IPAddress }}" node1)

    # node2 specific details
    local node2_pubkey=oJlDE1y9xxmR6CIEYCSJAN+8b/RK73TpBYixlFiBJDM=
    local node2_pvtkey=cGXbnP3WKIYbIbEyFpQ+kziNk/kHBM8VJhslEG8Uj1c=
    local node2_ip=$(sudo docker inspect --format "{{ .NetworkSettings.IPAddress }}" node2)

    # Create the new zone
    curl -L -X POST 'http://localhost:8080/zone' \
    -H 'Content-Type: application/json' \
    --data-raw '{
        "Name": "zone-blue",
        "Description": "Tenant - Zone Blue",
        "CIDR": "10.140.0.0/20"
    }'

    # Create jaywalk startup script for node1
    e2e-scripts/create-jaywalk-startup.sh ${node1_pubkey} ${node1_pvtkey} ${controller} ${node1_ip} ${controller_passwd} ${zone} jaywalk-run-node1.sh
    # Create jaywalk startup script for node2
    e2e-scripts/create-jaywalk-startup.sh ${node2_pubkey} ${node2_pvtkey} ${controller} ${node2_ip} ${controller_passwd} ${zone} jaywalk-run-node2.sh

    # STDOUT the scripts for debugging
    cat jaywalk-run-node1.sh
    cat jaywalk-run-node2.sh

    # Kill the jaywalk process on both nodes
    sudo docker exec node1 killall jaywalk
    sudo docker exec node2 killall jaywalk

    sudo docker cp ./jaywalk-run-node1.sh node1:/bin/jaywalk-run-node1.sh
    sudo docker cp ./jaywalk-run-node2.sh node2:/bin/jaywalk-run-node2.sh

    # Set permissions in the container
    sudo docker exec node1 chmod +x /bin/jaywalk-run-node1.sh
    sudo docker exec node2 chmod +x /bin/jaywalk-run-node2.sh

    # Start the agents on both nodes
    sudo docker exec node1 /bin/jaywalk-run-node1.sh &
    sudo docker exec node2 /bin/jaywalk-run-node2.sh &

    # Allow one second for the wg0 interface to readdress
    sleep 2

    # Check connectivity between node1 -> node2
    if sudo docker exec node1 ping -c 2 -w 2 $(sudo docker exec node2 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node1 failed to reach node2, e2e failed"
        exit 1
    fi
    # Check connectivity between node2 -> node1
    if sudo docker exec node2 ping -c 2 -w 2 $(sudo docker exec node1 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node2 failed to reach node1, e2e failed"
        exit 1
    fi
}
###########################################################################
# Description:                                                            #
# Run the following functions to test end to end connectivity between     #
# Wireguard interfaces in the container on interface wg0                 #
#                                                                         #
# Arguments:                                                              #
#   None                                                                  #
###########################################################################
install_docker
start_containers
start_supervisor
copy_binaries
verify_connectivity
setup_custom_zone_connectivity
verify_connectivity
