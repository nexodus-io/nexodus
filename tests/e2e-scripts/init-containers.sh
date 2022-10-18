#!/bin/bash
# fail the script if any errors are encountered
set -ex

start_containers() {
    ###########################################################################
    # Description:                                                            #
    # Start the redis broker instance, and two Docker edge nodes              #
    #                                                                         #
    # Arguments:                                                              #
    #   Arg1: Node Container Image                                            #
    ###########################################################################

    local node_image=${1}

    $DOCKER_COMPOSE up -d

    # allow for all services to come up and be ready
    # TODO: Replace with a proper healthcheck
    sleep 10

    # Start node1 (container image is generic until the cli stabilizes so no arguments, a script below builds the aircrew cmd)
    $DOCKER run -itd \
        --name=node1 \
        --net=jaywalking_default \
        --cap-add=SYS_MODULE \
        --cap-add=NET_ADMIN \
        --cap-add=NET_RAW \
        ${node_image}

    # Start node2post
    $DOCKER run -itd \
        --name=node2 \
        --net=jaywalking_default \
        --cap-add=SYS_MODULE \
        --cap-add=NET_ADMIN \
        --cap-add=NET_RAW \
        ${node_image}
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
    local controller=redis
    local controller_passwd=floofykittens
    local zone="00000000-0000-0000-0000-000000000000"

    # node1 specific details
    local node1_pubkey=AbZ1fPkCbjYAe9D61normbb7urAzMGaRMDVyR5Bmzz4=
    local node1_pvtkey=8GtvCMlUsFVoadj0B3Y3foy7QbKJB9vcq5R+Mpc7OlE=
    local node1_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.jaywalking_default.IPAddress }}" node1)

    # node2 specific details
    local node2_pubkey=oJlDE1y9xxmR6CIEYCSJAN+8b/RK73TpBYixlFiBJDM=
    local node2_pvtkey=cGXbnP3WKIYbIbEyFpQ+kziNk/kHBM8VJhslEG8Uj1c=
    local node2_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.jaywalking_default.IPAddress }}" node2)

    # Node-1 aircrew run default zone
    cat <<EOF > aircrew-run-node1.sh
#!/bin/bash
aircrew \
--public-key=${node1_pubkey} \
--private-key=${node1_pvtkey} \
--controller=${controller} \
--local-endpoint-ip=${node1_ip} \
--controller-password=${controller_passwd}
EOF

    # Node-2 aircrew run default zone
    cat <<EOF > aircrew-run-node2.sh
#!/bin/bash
aircrew \
--public-key=${node2_pubkey} \
--private-key=${node2_pvtkey} \
--controller=${controller} \
--local-endpoint-ip=${node2_ip} \
--controller-password=${controller_passwd}
EOF

    # STDOUT the run scripts for debugging
    echo "=== Displaying aircrew-run-node1.sh ==="
    cat aircrew-run-node1.sh
    echo "=== Displaying aircrew-run-node2.sh ==="
    cat aircrew-run-node2.sh

    # Copy binaries and scripts (copying the controltower even though we are running it on the VM instead of in a container)
    $DOCKER cp $(which aircrew) node1:/bin/aircrew
    $DOCKER cp $(which aircrew) node2:/bin/aircrew
    $DOCKER cp $(which controltower) node1:/bin/controltower
    $DOCKER cp $(which controltower) node2:/bin/controltower
    $DOCKER cp ./aircrew-run-node1.sh node1:/bin/aircrew-run-node1.sh
    $DOCKER cp ./aircrew-run-node2.sh node2:/bin/aircrew-run-node2.sh

    # Set permissions in the container
    $DOCKER exec node1 chmod +x /bin/aircrew-run-node1.sh
    $DOCKER exec node2 chmod +x /bin/aircrew-run-node2.sh

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/aircrew-run-node1.sh &
    $DOCKER exec node2 /bin/aircrew-run-node2.sh &
}

verify_connectivity() {
    ###########################################################################
    # Description:                                                            #
    # Verify the container can reach one another                              #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    # Allow for convergence
    sleep 4
    # Check connectivity between node1 -> node2
    if $DOCKER exec node1 ping -c 2 -w 2 $($DOCKER exec node2 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node1 failed to reach node2, e2e failed"
        exit 1
    fi
    # Check connectivity between node2 -> node1
    if $DOCKER exec node2 ping -c 2 -w 2 $($DOCKER exec node1 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node2 failed to reach node1, e2e failed"
        exit 1
    fi
}

setup_custom_zone_connectivity() {
    ###########################################################################
    # Description:                                                            #
    # Verify the zone api works and a custom sec zone                         #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    # Shared controller address
    local controller=redis
    local controller_passwd=floofykittens
    local node_pvtkey_file=/etc/wireguard/private.key

    # node1 specific details
    local node1_pubkey=AbZ1fPkCbjYAe9D61normbb7urAzMGaRMDVyR5Bmzz4=
    local node1_pvtkey=8GtvCMlUsFVoadj0B3Y3foy7QbKJB9vcq5R+Mpc7OlE=
    local node1_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.jaywalking_default.IPAddress }}" node1)

    # node2 specific details
    local node2_pubkey=oJlDE1y9xxmR6CIEYCSJAN+8b/RK73TpBYixlFiBJDM=
    local node2_pvtkey=cGXbnP3WKIYbIbEyFpQ+kziNk/kHBM8VJhslEG8Uj1c=
    local node2_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.jaywalking_default.IPAddress }}" node2)

    # Create the new zone
    local zone=$(curl -L -X POST 'http://localhost:8080/zones' \
    -H 'Content-Type: application/json' \
    --data-raw '{
        "Name": "zone-blue",
        "Description": "Tenant - Zone Blue",
        "CIDR": "10.140.0.0/20"
    }' | jq -r '.ID')

    # Create private key files for both nodes (new lines are there to validate the agent handles strip those out)
    echo -e  "$node1_pvtkey\n\n" | tee node1-private.key
    echo -e  "$node2_pvtkey\n\n" | tee node2-private.key

    # Node-1 aircrew run
    cat <<EOF > aircrew-run-node1.sh
#!/bin/bash
aircrew \
--public-key=${node1_pubkey} \
--private-key-file=/etc/wireguard/private.key \
--controller=${controller} \
--local-endpoint-ip=${node1_ip} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Node-2 aircrew run
    cat <<EOF > aircrew-run-node2.sh
#!/bin/bash
aircrew \
--public-key=${node2_pubkey} \
--private-key-file=/etc/wireguard/private.key \
--controller=${controller} \
--local-endpoint-ip=${node2_ip} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Kill the aircrew process on both nodes
    $DOCKER exec node1 killall aircrew
    $DOCKER exec node2 killall aircrew

    # STDOUT the run scripts for debugging
    echo "=== Displaying aircrew-run-node1.sh ==="
    cat aircrew-run-node1.sh
    echo "=== Displaying aircrew-run-node2.sh ==="
    cat aircrew-run-node2.sh

    $DOCKER cp ./aircrew-run-node1.sh node1:/bin/aircrew-run-node1.sh
    $DOCKER cp ./aircrew-run-node2.sh node2:/bin/aircrew-run-node2.sh
    $DOCKER cp ./node1-private.key node1:/etc/wireguard/private.key
    $DOCKER cp ./node2-private.key node2:/etc/wireguard/private.key

    # Set permissions in the container
    $DOCKER exec node1 chmod +x /bin/aircrew-run-node1.sh
    $DOCKER exec node2 chmod +x /bin/aircrew-run-node2.sh

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/aircrew-run-node1.sh &
    $DOCKER exec node2 /bin/aircrew-run-node2.sh &

    # Allow two seconds for the wg0 interface to readdress
    sleep 2
}

setup_custom_second_zone_connectivity() {
    ###########################################################################
    # Description:                                                            #
    # Verify a second custom zone can be created and connected with no        #
    # errors using a different key pair as prior tests                        #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    # Shared controller address
    local controller=redis
    local controller_passwd=floofykittens
    local node_pvtkey_file=/etc/wireguard/private.key

    # node1 specific details
    local node1_pubkey=M+BTP8LbMikKLufoTTI7tPL5Jf3SHhNki6SXEXa5Uic=
    local node1_pvtkey=4OXhMZdzodfOrmWvZyJRfiDEm+FJSwaEMI4co0XRP18=
    local node1_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.jaywalking_default.IPAddress }}" node1)

    # node2 specific details
    local node2_pubkey=DUQ+TxqMya3YgRd1eXW/Tcg2+6wIX5uwEKqv6lOScAs=
    local node2_pvtkey=WBydF4bEIs/uSR06hrsGa4vhgNxgR6rmR68CyOHMK18=
    local node2_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.jaywalking_default.IPAddress }}" node2)

    # Create the new zone with a CGNAT range
    local zone=$(curl -L -X POST 'http://localhost:8080/zones' \
    -H 'Content-Type: application/json' \
    --data-raw '{
        "Name": "zone-red",
        "Description": "Tenant - Zone Red",
        "CIDR": "100.64.0.0/20"
    }' | jq -r '.ID')

    # Create private key files for both nodes (new lines are there to validate the agent handles strip those out)
    echo -e  "\n$node1_pvtkey" | tee node1-private.key
    echo -e  "\n$node2_pvtkey" | tee node2-private.key

    # Node-1 aircrew run
    cat <<EOF > aircrew-run-node1.sh
#!/bin/bash
aircrew \
--public-key=${node1_pubkey} \
--private-key-file=/etc/wireguard/private.key \
--controller=${controller} \
--local-endpoint-ip=${node1_ip} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Node-2 aircrew run
    cat <<EOF > aircrew-run-node2.sh
#!/bin/bash
aircrew \
--public-key=${node2_pubkey} \
--private-key-file=/etc/wireguard/private.key \
--controller=${controller} \
--local-endpoint-ip=${node2_ip} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Kill the aircrew process on both nodes
    $DOCKER exec node1 killall aircrew
    $DOCKER exec node2 killall aircrew

    # STDOUT the run scripts for debugging
    echo "=== Displaying aircrew-run-node1.sh ==="
    cat aircrew-run-node1.sh
    echo "=== Displaying aircrew-run-node2.sh ==="
    cat aircrew-run-node2.sh

    $DOCKER cp ./aircrew-run-node1.sh node1:/bin/aircrew-run-node1.sh
    $DOCKER cp ./aircrew-run-node2.sh node2:/bin/aircrew-run-node2.sh
    $DOCKER cp ./node1-private.key node1:/etc/wireguard/private.key
    $DOCKER cp ./node2-private.key node2:/etc/wireguard/private.key

    # Set permissions in the container
    $DOCKER exec node1 chmod +x /bin/aircrew-run-node1.sh
    $DOCKER exec node2 chmod +x /bin/aircrew-run-node2.sh

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/aircrew-run-node1.sh &
    $DOCKER exec node2 /bin/aircrew-run-node2.sh &

    # Allow two seconds for the wg0 interface to readdress
    sleep 2
}


setup_child_prefix_connectivity() {
    ###########################################################################
    # Description:                                                            #
    # Verify a child-prefix and request-ip can be created and add a loopback  #
    # on each node in the child prefix cidr and verify connectivity           #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    # Shared controller address
    local controller=redis
    local controller_passwd=floofykittens
    local node_pvtkey_file=/etc/wireguard/private.key

    # node1 specific details
    local requested_ip_node1=192.168.200.100
    local child_prefix_node1=172.20.1.0/24
    local node1_pubkey=M+BTP8LbMikKLufoTTI7tPL5Jf3SHhNki6SXEXa5Uic=
    local node1_pvtkey=4OXhMZdzodfOrmWvZyJRfiDEm+FJSwaEMI4co0XRP18=
    local node1_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.jaywalking_default.IPAddress }}" node1)

    # node2 specific details
    local requested_ip_node2=192.168.200.200
    local child_prefix_node2=172.20.3.0/24
    local node2_pubkey=DUQ+TxqMya3YgRd1eXW/Tcg2+6wIX5uwEKqv6lOScAs=
    local node2_pvtkey=WBydF4bEIs/uSR06hrsGa4vhgNxgR6rmR68CyOHMK18=
    local node2_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.jaywalking_default.IPAddress }}" node2)

    # Delete the ipam storage in the case the run has re-run since we dont overwrite existing child-prefix
    rm -rf prefix-test.json

    # Create the new zone with a CGNAT range
    local zone=$(curl -L -X POST 'http://localhost:8080/zones' \
    -H 'Content-Type: application/json' \
    --data-raw '{
        "Name": "prefix-test",
        "Description": "Tenant - Zone prefix-test",
        "CIDR": "192.168.200.0/24"
    }' | jq -r '.ID')

    # Create private key files for both nodes (new lines are there to validate the agent handles strip those out)
    echo -e  "\n$node1_pvtkey" | tee node1-private.key
    echo -e  "\n$node2_pvtkey" | tee node2-private.key

    # Kill the aircrew process on both nodes
    $DOCKER exec node1 killall aircrew
    $DOCKER exec node2 killall aircrew

    # Node-1 aircrew run
    cat <<EOF > aircrew-run-node1.sh
#!/bin/bash
    aircrew --public-key=${node1_pubkey} \
    --private-key-file=/etc/wireguard/private.key  \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --child-prefix=${child_prefix_node1} \
    --internal-network \
    --request-ip=${requested_ip_node1} \
    --zone=${zone}
EOF

    # Node-2 aircrew run
    cat <<EOF > aircrew-run-node2.sh
#!/bin/bash
    aircrew --public-key=${node2_pubkey} \
    --private-key-file=/etc/wireguard/private.key  \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --child-prefix=${child_prefix_node2} \
    --request-ip=${requested_ip_node2} \
    --internal-network \
    --zone=${zone}
EOF

    # STDOUT the run scripts for debugging
    echo "=== Displaying aircrew-run-node1.sh ==="
    cat aircrew-run-node1.sh
    echo "=== Displaying aircrew-run-node2.sh ==="
    cat aircrew-run-node2.sh

    # Copy files to the containers
    $DOCKER cp ./aircrew-run-node1.sh node1:/bin/aircrew-run-node1.sh
    $DOCKER cp ./aircrew-run-node2.sh node2:/bin/aircrew-run-node2.sh
    $DOCKER cp ./node1-private.key node1:/etc/wireguard/private.key
    $DOCKER cp ./node2-private.key node2:/etc/wireguard/private.key

    # Set permissions in the container
    $DOCKER exec node1 chmod +x /bin/aircrew-run-node1.sh
    $DOCKER exec node2 chmod +x /bin/aircrew-run-node2.sh

    # Add loopback addresses the are in the child-prefix cidr range
    $DOCKER exec node1 ip addr add 172.20.1.10/32 dev lo
    $DOCKER exec node2 ip addr add 172.20.3.10/32 dev lo

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/aircrew-run-node1.sh &
    $DOCKER exec node2 /bin/aircrew-run-node2.sh &

    # Allow four seconds for the wg0 interface to readdress
    sleep 4
    
    # Check connectivity between node1  child prefix loopback-> node2 child prefix loopback
    if $DOCKER exec node1 ping -c 2 -w 2 172.20.3.10; then
        echo "peer node loopbacks successfully communicated"
    else
        echo "node1 failed to reach node2 loopback, e2e failed"
        exit 1
    fi
    # Check connectivity between node2 child prefix loopback -> node1 child prefix loopback
    if $DOCKER exec node2 ping -c 2 -w 2 172.20.1.10; then
        echo "peer node loopbacks successfully communicated"
    else
        echo "node2 failed to reach node1 loopback, e2e failed"
        exit 1
    fi
}

clean_nodes() {
    ###########################################################################
    # Description:                                                            #
    # Clean up the nodes in between tests                                     #
    # Wireguard interfaces in the container on interface wg0                  #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################

    $DOCKER exec node1 ip link del wg0
    $DOCKER exec node2 ip link del wg0
}


###########################################################################
# Description:                                                            #
# Run the following functions to test end to end connectivity between     #
# Wireguard interfaces in the container on interface wg0                  #
                                                                          #
###########################################################################

while getopts "o:" flag; do
    case "${flag}" in
    o) os="${OPTARG}" ;;
    esac
done

if [ -z "$DOCKER" ]; then
    DOCKER=docker
fi
if [ -z "$DOCKER_COMPOSE" ]; then
    DOCKER_COMPOSE=docker-compose
fi

echo -e "Job running with OS Image: ${os}"

start_containers ${os}
copy_binaries
verify_connectivity
clean_nodes
setup_custom_zone_connectivity
verify_connectivity
clean_nodes
setup_custom_second_zone_connectivity
verify_connectivity
clean_nodes
setup_child_prefix_connectivity
verify_connectivity
clean_nodes

echo "e2e completed"