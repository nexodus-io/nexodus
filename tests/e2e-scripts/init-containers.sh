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

    # Start node1 (container image is generic until the cli stabilizes so no arguments, a script below builds the apex cmd)
    $DOCKER run -itd \
        --name=node1 \
        --net=apex_default \
        --cap-add=SYS_MODULE \
        --cap-add=NET_ADMIN \
        --cap-add=NET_RAW \
        ${node_image}

    # Start node2post
    $DOCKER run -itd \
        --name=node2 \
        --net=apex_default \
        --cap-add=SYS_MODULE \
        --cap-add=NET_ADMIN \
        --cap-add=NET_RAW \
        ${node_image}

    # Start node3post
    $DOCKER run -itd \
        --name=node3 \
        --net=apex_default \
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
    local node1_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node1)

    # node2 specific details
    local node2_pubkey=oJlDE1y9xxmR6CIEYCSJAN+8b/RK73TpBYixlFiBJDM=
    local node2_pvtkey=cGXbnP3WKIYbIbEyFpQ+kziNk/kHBM8VJhslEG8Uj1c=
    local node2_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node2)

    # Node-1 apex run default zone
    cat <<EOF > apex-run-node1.sh
#!/bin/bash
apex \
--public-key=${node1_pubkey} \
--private-key=${node1_pvtkey} \
--controller=${controller} \
--local-endpoint-ip=${node1_ip} \
--controller-password=${controller_passwd}
EOF

    # Node-2 apex run default zone
    cat <<EOF > apex-run-node2.sh
#!/bin/bash
apex \
--public-key=${node2_pubkey} \
--private-key=${node2_pvtkey} \
--controller=${controller} \
--local-endpoint-ip=${node2_ip} \
--controller-password=${controller_passwd}
EOF

    # STDOUT the run scripts for debugging
    echo "=== Displaying apex-run-node1.sh ==="
    cat apex-run-node1.sh
    echo "=== Displaying apex-run-node2.sh ==="
    cat apex-run-node2.sh

    # Copy binaries and scripts (copying the controller even though we are running it on the VM instead of in a container)
    $DOCKER cp $(which apex) node1:/bin/apex
    $DOCKER cp $(which apex) node2:/bin/apex
    $DOCKER cp $(which apex) node3:/bin/apex

    # Deploy run scripts to nodes
    $DOCKER cp ./apex-run-node1.sh node1:/bin/apex-run-node1.sh
    $DOCKER cp ./apex-run-node2.sh node2:/bin/apex-run-node2.sh

    # Set permissions in the container
    $DOCKER exec node1 chmod +x /bin/apex-run-node1.sh
    $DOCKER exec node2 chmod +x /bin/apex-run-node2.sh

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/apex-run-node1.sh &
    $DOCKER exec node2 /bin/apex-run-node2.sh &
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
    local node1_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node1)

    # node2 specific details
    local node2_pubkey=oJlDE1y9xxmR6CIEYCSJAN+8b/RK73TpBYixlFiBJDM=
    local node2_pvtkey=cGXbnP3WKIYbIbEyFpQ+kziNk/kHBM8VJhslEG8Uj1c=
    local node2_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node2)

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

    # Node-1 apex run
    cat <<EOF > apex-run-node1.sh
#!/bin/bash
apex \
--public-key=${node1_pubkey} \
--private-key-file=/etc/wireguard/private.key \
--controller=${controller} \
--local-endpoint-ip=${node1_ip} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Node-2 apex run
    cat <<EOF > apex-run-node2.sh
#!/bin/bash
apex \
--public-key=${node2_pubkey} \
--private-key-file=/etc/wireguard/private.key \
--controller=${controller} \
--local-endpoint-ip=${node2_ip} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Kill the apex process on both nodes
    $DOCKER exec node1 killall apex
    $DOCKER exec node2 killall apex

    # STDOUT the run scripts for debugging
    echo "=== Displaying apex-run-node1.sh ==="
    cat apex-run-node1.sh
    echo "=== Displaying apex-run-node2.sh ==="
    cat apex-run-node2.sh

    $DOCKER cp ./apex-run-node1.sh node1:/bin/apex-run-node1.sh
    $DOCKER cp ./apex-run-node2.sh node2:/bin/apex-run-node2.sh
    $DOCKER cp ./node1-private.key node1:/etc/wireguard/private.key
    $DOCKER cp ./node2-private.key node2:/etc/wireguard/private.key

    # Set permissions in the container
    $DOCKER exec node1 chmod +x /bin/apex-run-node1.sh
    $DOCKER exec node2 chmod +x /bin/apex-run-node2.sh

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/apex-run-node1.sh &
    $DOCKER exec node2 /bin/apex-run-node2.sh &

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
    local node1_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node1)

    # node2 specific details
    local node2_pubkey=DUQ+TxqMya3YgRd1eXW/Tcg2+6wIX5uwEKqv6lOScAs=
    local node2_pvtkey=WBydF4bEIs/uSR06hrsGa4vhgNxgR6rmR68CyOHMK18=
    local node2_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node2)

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

    # Node-1 apex run
    cat <<EOF > apex-run-node1.sh
#!/bin/bash
apex \
--public-key=${node1_pubkey} \
--private-key-file=/etc/wireguard/private.key \
--controller=${controller} \
--local-endpoint-ip=${node1_ip} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Node-2 apex run
    cat <<EOF > apex-run-node2.sh
#!/bin/bash
apex \
--public-key=${node2_pubkey} \
--private-key-file=/etc/wireguard/private.key \
--controller=${controller} \
--local-endpoint-ip=${node2_ip} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Kill the apex process on both nodes
    $DOCKER exec node1 killall apex
    $DOCKER exec node2 killall apex

    # STDOUT the run scripts for debugging
    echo "=== Displaying apex-run-node1.sh ==="
    cat apex-run-node1.sh
    echo "=== Displaying apex-run-node2.sh ==="
    cat apex-run-node2.sh

    $DOCKER cp ./apex-run-node1.sh node1:/bin/apex-run-node1.sh
    $DOCKER cp ./apex-run-node2.sh node2:/bin/apex-run-node2.sh
    $DOCKER cp ./node1-private.key node1:/etc/wireguard/private.key
    $DOCKER cp ./node2-private.key node2:/etc/wireguard/private.key

    # Set permissions in the container
    $DOCKER exec node1 chmod +x /bin/apex-run-node1.sh
    $DOCKER exec node2 chmod +x /bin/apex-run-node2.sh

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/apex-run-node1.sh &
    $DOCKER exec node2 /bin/apex-run-node2.sh &

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
    local requested_ip_node1=192.168.200.101
    local child_prefix_node1=172.20.1.0/24
    local node1_loopback=172.20.1.10
    local node1_pubkey=M+BTP8LbMikKLufoTTI7tPL5Jf3SHhNki6SXEXa5Uic=
    local node1_pvtkey=4OXhMZdzodfOrmWvZyJRfiDEm+FJSwaEMI4co0XRP18=
    local node1_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node1)

    # node2 specific details
    local requested_ip_node2=192.168.200.102
    local child_prefix_node2=172.20.2.0/24
    local node2_loopback=172.20.2.10
    local node2_pubkey=DUQ+TxqMya3YgRd1eXW/Tcg2+6wIX5uwEKqv6lOScAs=
    local node2_pvtkey=WBydF4bEIs/uSR06hrsGa4vhgNxgR6rmR68CyOHMK18=
    local node2_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node2)

    # Create the new zone with a RFC1918 prefix
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

    # Kill the apex process on both nodes
    $DOCKER exec node1 killall apex
    $DOCKER exec node2 killall apex

    # Node-1 apex run
    cat <<EOF > apex-run-node1.sh
#!/bin/bash
    apex --public-key=${node1_pubkey} \
    --private-key-file=/etc/wireguard/private.key  \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --child-prefix=${child_prefix_node1} \
    --internal-network \
    --request-ip=${requested_ip_node1} \
    --zone=${zone}
EOF

    # Node-2 apex run
    cat <<EOF > apex-run-node2.sh
#!/bin/bash
    apex --public-key=${node2_pubkey} \
    --private-key-file=/etc/wireguard/private.key  \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --child-prefix=${child_prefix_node2} \
    --request-ip=${requested_ip_node2} \
    --internal-network \
    --zone=${zone}
EOF

    # STDOUT the run scripts for debugging
    echo "=== Displaying apex-run-node1.sh ==="
    cat apex-run-node1.sh
    echo "=== Displaying apex-run-node2.sh ==="
    cat apex-run-node2.sh

    # Copy files to the containers
    $DOCKER cp ./apex-run-node1.sh node1:/bin/apex-run-node1.sh
    $DOCKER cp ./apex-run-node2.sh node2:/bin/apex-run-node2.sh
    $DOCKER cp ./node1-private.key node1:/etc/wireguard/private.key
    $DOCKER cp ./node2-private.key node2:/etc/wireguard/private.key

    # Set permissions in the container
    $DOCKER exec node1 chmod +x /bin/apex-run-node1.sh
    $DOCKER exec node2 chmod +x /bin/apex-run-node2.sh

    # Add loopback addresses the are in the child-prefix cidr range
    $DOCKER exec node1 ip addr add ${node1_loopback}/32 dev lo
    $DOCKER exec node2 ip addr add ${node2_loopback}/32 dev lo

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/apex-run-node1.sh &
    $DOCKER exec node2 /bin/apex-run-node2.sh &

    # Allow four seconds for the wg0 interface to readdress
    sleep 4
    
    # Check connectivity between node1  child prefix loopback-> node2 child prefix loopback
    if $DOCKER exec node1 ping -c 2 -w 2 ${node2_loopback}; then
        echo "peer node loopbacks successfully communicated"
    else
        echo "node1 failed to reach node2 loopback, e2e failed"
        exit 1
    fi
    # Check connectivity between node2 child prefix loopback -> node1 child prefix loopback
    if $DOCKER exec node2 ping -c 2 -w 2 ${node1_loopback}; then
        echo "peer node loopbacks successfully communicated"
    else
        echo "node2 failed to reach node1 loopback, e2e failed"
        exit 1
    fi

    # Check connectivity between node1 requested-ip -> node2 requested-ip
    if $DOCKER exec node1 ping -c 2 -w 2 ${requested_ip_node2}; then
        echo "peer node loopbacks successfully communicated"
    else
        echo "node1 failed to reach node2 loopback, e2e failed"
        exit 1
    fi
    # Check connectivity between node2 requested-ip -> node1 requested-ip
    if $DOCKER exec node2 ping -c 2 -w 2 ${requested_ip_node1}; then
        echo "peer node loopbacks successfully communicated"
    else
        echo "node2 failed to reach node1 loopback, e2e failed"
        exit 1
    fi

    ###########################################################################
    # Description:                                                            #
    # This next section changes elements in the node and ensures the nodes    #
    # properly update one each others peer record in the wg config            #                                                             #
    ###########################################################################
    # Kill the apex process on both nodes
    $DOCKER exec node1 killall apex
    $DOCKER exec node2 killall apex

    # Now change out some elements and make sure the controller updates the device's configuration to the peers
    local new_child_prefix_node1=172.21.220.0/24
    local new_child_prefix_node2=172.21.221.0/24
    local new_requested_ip_node1=192.168.200.220
    local new_requested_ip_node2=192.168.200.221
    local new_node1_loopback=172.21.220.10
    local new_node2_loopback=172.21.221.10

    # Node-1 apex run
    cat <<EOF > apex-run-node1-cycle2.sh
#!/bin/bash
    apex --public-key=${node1_pubkey} \
    --private-key-file=/etc/wireguard/private.key  \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --child-prefix=${new_child_prefix_node1} \
    --local-endpoint-ip=${node1_ip} \
    --request-ip=${new_requested_ip_node1} \
    --zone=${zone}
EOF

    # Node-2 apex run
    cat <<EOF > apex-run-node2-cycle2.sh
#!/bin/bash
    apex --public-key=${node2_pubkey} \
    --private-key-file=/etc/wireguard/private.key  \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --child-prefix=${new_child_prefix_node2} \
    --request-ip=${new_requested_ip_node2} \
    --local-endpoint-ip=${node2_ip} \
    --zone=${zone}
EOF

    # STDOUT the run scripts for debugging
    echo "=== Displaying apex-run-node1.sh ==="
    cat apex-run-node1-cycle2.sh
    echo "=== Displaying apex-run-node2.sh ==="
    cat apex-run-node2-cycle2.sh

    # Copy files to the containers
    $DOCKER cp ./apex-run-node1-cycle2.sh node1:/bin/apex-run-node1-cycle2.sh
    $DOCKER cp ./apex-run-node2-cycle2.sh node2:/bin/apex-run-node2-cycle2.sh
    $DOCKER cp ./node1-private.key node1:/etc/wireguard/private.key
    $DOCKER cp ./node2-private.key node2:/etc/wireguard/private.key

    # Set permissions in the container
    $DOCKER exec node1 chmod +x /bin/apex-run-node1-cycle2.sh
    $DOCKER exec node2 chmod +x /bin/apex-run-node2-cycle2.sh

    # Add loopback addresses the are in the child-prefix cidr range
    $DOCKER exec node1 ip addr add ${new_node1_loopback}/32 dev lo
    $DOCKER exec node2 ip addr add ${new_node2_loopback}/32 dev lo

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/apex-run-node1-cycle2.sh &
    $DOCKER exec node2 /bin/apex-run-node2-cycle2.sh &

    # Allow five seconds for the wg0 interface to readdress
    sleep 5

    # Check connectivity between node1  child prefix loopback-> node2 child prefix loopback
    if $DOCKER exec node1 ping -c 2 -w 2 ${new_node1_loopback}; then
        echo "peer node loopbacks successfully communicated"
    else
        echo "node1 failed to reach the updated node2 loopback, e2e failed"
        exit 1
    fi
    # Check connectivity between node2 child prefix loopback -> node1 child prefix loopback
    if $DOCKER exec node2 ping -c 2 -w 2 ${new_node2_loopback}; then
        echo "peer node loopbacks successfully communicated"
    else
        echo "node2 failed to reach the updated node1 loopback, e2e failed"
        exit 1
    fi
    # Check connectivity between node1 new_requested-ip -> node2 new_requested-ip
    if $DOCKER exec node1 ping -c 2 -w 2 ${new_requested_ip_node2}; then
        echo "peer node wg0 successfully communicated"
    else
        echo "node1 failed to reach node2 wg0, e2e failed"
        exit 1
    fi
    # Check connectivity between node2 new_requested-ip -> node1 new_requested-ip
    if $DOCKER exec node2 ping -c 2 -w 2 ${new_requested_ip_node1}; then
        echo "peer node wg0 successfully communicated"
    else
        echo "node2 failed to reach node1 wg0, e2e failed"
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


setup_hub_spoke_connectivity() {
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
    local zone=hub-spoke-zone
    local node_pvtkey_file=/etc/wireguard/private.key

    # node1 specific details
    local node1_pubkey=M+BTP8LbMikKLufoTTI7tPL5Jf3SHhNki6SXEXa5Uic=
    local node1_pvtkey=4OXhMZdzodfOrmWvZyJRfiDEm+FJSwaEMI4co0XRP18=
    local node1_ip=$(sudo $DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node1)

    # node2 specific details
    local node2_pubkey=DUQ+TxqMya3YgRd1eXW/Tcg2+6wIX5uwEKqv6lOScAs=
    local node2_pvtkey=WBydF4bEIs/uSR06hrsGa4vhgNxgR6rmR68CyOHMK18=
    local node2_ip=$(sudo $DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node2)

    # node3 specific details
    local node3_pubkey=305lmZr0lFYy3E1S6e/GLCup300W5T4mOMnF9SKjmzc=
    local node3_pvtkey=CCWJ1RfGdFxq9nBCYLa33I6B6IR9EPkMGnyb5gnJ+FI=
    local node3_ip=$(sudo $DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node3)

    # Create the new zone
    local zone=$(curl -L -X POST 'http://localhost:8080/zones' \
    -H 'Content-Type: application/json' \
    --data-raw '{
        "Name": "hub-spoke-zone",
        "Description": "Hub/Spoke Zone",
        "CIDR": "10.89.0.0/27",
        "Hub-Zone": true
    }' | jq -r '.ID')

    # Create private key files for both nodes
    echo -e  "$node1_pvtkey" | tee node1-private.key
    echo -e  "$node2_pvtkey" | tee node2-private.key
    echo -e  "$node3_pvtkey" | tee node3-private.key

    # Kill the apex process on both nodes (no process running on node3 yet)
    sudo $DOCKER exec node1 killall apex
    sudo $DOCKER exec node2 killall apex

    # Node-1 apex run
    cat <<EOF > apex-run-node1.sh
#!/bin/bash
    apex --public-key=${node1_pubkey} \
    --private-key-file=/etc/wireguard/private.key \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --internal-network \
    --hub-router \
    --zone=${zone}
EOF

    # Node-2 apex run
    cat <<EOF > apex-run-node2.sh
#!/bin/bash
    apex --public-key=${node2_pubkey} \
    --private-key-file=/etc/wireguard/private.key \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --internal-network \
    --zone=${zone}
EOF

    # Node-3 apex run
    cat <<EOF > apex-run-node3.sh
#!/bin/bash
    apex --public-key=${node3_pubkey} \
    --private-key-file=/etc/wireguard/private.key \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --internal-network \
    --zone=${zone}
EOF

    # STDOUT the run scripts for debugging
    echo "=== Displaying apex-run-node1.sh ==="
    cat apex-run-node1.sh
    echo "=== Displaying apex-run-node2.sh ==="
    cat apex-run-node2.sh
    echo "=== Displaying apex-run-node3.sh ==="
    cat apex-run-node3.sh

    # Copy files to the containers
    sudo $DOCKER cp ./apex-run-node1.sh node1:/bin/apex-run-node1.sh
    sudo $DOCKER cp ./apex-run-node2.sh node2:/bin/apex-run-node2.sh
    sudo $DOCKER cp ./apex-run-node3.sh node3:/bin/apex-run-node3.sh
    sudo $DOCKER cp ./node1-private.key node1:/etc/wireguard/private.key
    sudo $DOCKER cp ./node2-private.key node2:/etc/wireguard/private.key
    sudo $DOCKER cp ./node3-private.key node3:/etc/wireguard/private.key

    # Set permissions in the container
    sudo $DOCKER exec node1 chmod +x /bin/apex-run-node1.sh
    sudo $DOCKER exec node2 chmod +x /bin/apex-run-node2.sh
    sudo $DOCKER exec node3 chmod +x /bin/apex-run-node3.sh

    # Start the agents on all 3 nodes nodes (currently the hub-router needs to be spun up first)
    sudo $DOCKER exec node1 /bin/apex-run-node1.sh &
    sleep 5
    sudo $DOCKER exec node2 /bin/apex-run-node2.sh &
    sudo $DOCKER exec node3 /bin/apex-run-node3.sh &

    # Allow four seconds for the wg0 interface to readdress
    sleep 4

    # Check connectivity between node3 -> node1
    if sudo $DOCKER exec node3 ping -c 2 -w 2 $(sudo $DOCKER exec node1 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node3 failed to reach node1, e2e failed"
        exit 1
    fi
    # Check connectivity between node3 -> node2
    if sudo $DOCKER exec node3 ping -c 2 -w 2 $(sudo $DOCKER exec node2 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node3 failed to reach node2, e2e failed"
        exit 1
    fi

    # Check connectivity between node1 -> node3
    if sudo $DOCKER exec node1 ping -c 2 -w 2 $(sudo $DOCKER exec node3 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node1 failed to reach node3, e2e failed"
        exit 1
    fi
}

###########################################################################
# Description:                                                            #
# This test will cycle configurations to ensure database entries are      #
# work as expected and the configuration parsing works as intended        #
                                                                          #
###########################################################################
cycle_configurations(){
    local controller=redis
    local controller_passwd=floofykittens
    local node_pvtkey_file=/etc/wireguard/private.key

    # node1 specific details
    local node1_pubkey=bBAtxEphKIl8lXR3SU88d9slSxlyxmHxLQpHw3oBegc=
    local node1_pvtkey=wLN//bU622CxzFRH3t2V40aHurYW7Ad/8pc7wCMlS2g=
    local node1_ip=$(sudo $DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node1)

    # node2 specific details
    local node2_pubkey=J6SnyIt2cCgPLGEWvoZ6+OB4uNnl9QKejCE+HU9qn2Q=
    local node2_pvtkey=0NcaqbRNfixztY6izBC2B2NNrIpjj+hAlZLbp8H0NXI=
    local node2_ip=$(sudo $DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node2)

    # node3 specific details
    local node3_pubkey=oJlDE1y9xxmR6CIEYCSJAN+8b/RK73TpBYixlFiBJDM=
    local node3_pvtkey=cGXbnP3WKIYbIbEyFpQ+kziNk/kHBM8VJhslEG8Uj1c=
    local node3_ip=$(sudo $DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node3)

    echo -e  "$node1_pvtkey" | tee node1-private.key
    echo -e  "$node2_pvtkey" | tee node2-private.key
    echo -e  "$node3_pvtkey" | tee node3-private.key

    # Copy key file to the nodes
    sudo $DOCKER cp ./node1-private.key node1:/etc/wireguard/private.key
    sudo $DOCKER cp ./node2-private.key node2:/etc/wireguard/private.key
    sudo $DOCKER cp ./node3-private.key node3:/etc/wireguard/private.key

    # Create the new zone
    local zone=$(curl -L -X POST 'http://localhost:8080/zones' \
    -H 'Content-Type: application/json' \
    --data-raw '{
        "Name": "cycle-zone",
        "Description": "stress tester",
        "CIDR": "10.220.0.0/24"
    }' | jq -r '.ID')

    # Create configurations for three nodes
    for i in {1..2}
    do
        cat <<EOF > apex-run-node1-cycle${i}.sh
#!/bin/bash
    apex --public-key=${node1_pubkey} \
    --private-key-file=/etc/wireguard/private.key \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --local-endpoint-ip=${node1_ip} \
    --request-ip=10.220.10.${i} \
    --zone=${zone}
EOF

        cat <<EOF > apex-run-node2-cycle${i}.sh
#!/bin/bash
    apex --public-key=${node2_pubkey} \
    --private-key-file=/etc/wireguard/private.key \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --local-endpoint-ip=${node2_ip} \
    --request-ip=10.220.20.${i} \
    --zone=${zone}
EOF

        cat <<EOF > apex-run-node3-cycle${i}.sh
#!/bin/bash
    apex --public-key=${node3_pubkey} \
    --private-key-file=/etc/wireguard/private.key \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --local-endpoint-ip=${node3_ip} \
    --request-ip=10.220.30.${i} \
    --zone=${zone}
EOF
    done

    # Copy files to the containers
    sudo $DOCKER cp ./apex-run-node1-cycle1.sh node1:/bin/apex-run-node1-cycle1.sh
    sudo $DOCKER cp ./apex-run-node2-cycle1.sh node2:/bin/apex-run-node2-cycle1.sh
    sudo $DOCKER cp ./apex-run-node3-cycle1.sh node3:/bin/apex-run-node3-cycle1.sh

    # Set permissions in the container
    sudo $DOCKER exec node1 chmod +x /bin/apex-run-node1-cycle1.sh
    sudo $DOCKER exec node2 chmod +x /bin/apex-run-node2-cycle1.sh
    sudo $DOCKER exec node3 chmod +x /bin/apex-run-node3-cycle1.sh

    # Start the agents on all 3 nodes nodes (currently the hub-router needs to be spun up first)
    sudo $DOCKER exec node1 /bin/apex-run-node1-cycle1.sh &
    sudo $DOCKER exec node2 /bin/apex-run-node2-cycle1.sh &
    sudo $DOCKER exec node3 /bin/apex-run-node3-cycle1.sh &

    sleep 10
    # Check connectivity between node3 -> node1
    if sudo $DOCKER exec node3 ping -c 2 -w 2 $(sudo $DOCKER exec node1 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node3 failed to reach node1, e2e failed"
        exit 1
    fi
    # Check connectivity between node3 -> node2
    if sudo $DOCKER exec node3 ping -c 2 -w 2 $(sudo $DOCKER exec node2 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node3 failed to reach node2, e2e failed"
        exit 1
    fi
    # Check connectivity between node1 -> node3
    if sudo $DOCKER exec node1 ping -c 2 -w 2 $(sudo $DOCKER exec node3 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node1 failed to reach node3, e2e failed"
        exit 1
    fi
    sudo $DOCKER exec node1 killall apex
    sudo $DOCKER exec node2 killall apex
    sudo $DOCKER exec node3 killall apex

    # Count the occurrences of [Peer] in wg0.conf (should be 2 for a 3 node peering)
    wg_conf_peer_count=$(sudo $DOCKER exec node1 grep Peer /etc/wireguard/wg0.conf | wc -l)
    if ((wg_conf_peer_count != 2)); then
      echo "The peer count in wg0.conf  on node1 should be 2"
    fi
    wg_conf_peer_count=$(sudo $DOCKER exec node2 grep Peer /etc/wireguard/wg0.conf | wc -l)
    if ((wg_conf_peer_count != 2)); then
      echo "The peer count in wg0.conf  on node2 should be 2"
    fi
    wg_conf_peer_count=$(sudo $DOCKER exec node2 grep Peer /etc/wireguard/wg0.conf | wc -l)
    if ((wg_conf_peer_count != 2)); then
      echo "The peer count in wg0.conf on node3 should be 2"
    fi

    # Copy files to the containers
    sudo $DOCKER cp ./apex-run-node1-cycle2.sh node1:/bin/apex-run-node1-cycle2.sh
    sudo $DOCKER cp ./apex-run-node2-cycle2.sh node2:/bin/apex-run-node2-cycle2.sh
    sudo $DOCKER cp ./apex-run-node3-cycle2.sh node3:/bin/apex-run-node3-cycle2.sh

    # Set permissions in the container
    sudo $DOCKER exec node1 chmod +x /bin/apex-run-node1-cycle2.sh
    sudo $DOCKER exec node2 chmod +x /bin/apex-run-node2-cycle2.sh
    sudo $DOCKER exec node3 chmod +x /bin/apex-run-node3-cycle2.sh

    # Start the agents on all 3 nodes nodes (currently the hub-router needs to be spun up first)
    sudo $DOCKER exec node1 /bin/apex-run-node1-cycle2.sh &
    sudo $DOCKER exec node2 /bin/apex-run-node2-cycle2.sh &
    sudo $DOCKER exec node3 /bin/apex-run-node3-cycle2.sh &

    sleep 10
    # Check connectivity between node3 -> node1
    if sudo $DOCKER exec node3 ping -c 2 -w 2 $(sudo $DOCKER exec node1 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node3 failed to reach node1, e2e failed"
        exit 1
    fi
    # Check connectivity between node3 -> node2
    if sudo $DOCKER exec node3 ping -c 2 -w 2 $(sudo $DOCKER exec node2 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node3 failed to reach node2, e2e failed"
        exit 1
    fi
    # Check connectivity between node1 -> node3
    if sudo $DOCKER exec node1 ping -c 2 -w 2 $(sudo $DOCKER exec node3 ip --brief address show wg0 | awk '{print $3}' | cut -d "/" -f1); then
        echo "peer nodes successfully communicated"
    else
        echo "node1 failed to reach node3, e2e failed"
        exit 1
    fi
    sudo $DOCKER exec node1 killall apex
    sudo $DOCKER exec node2 killall apex
    sudo $DOCKER exec node3 killall apex

    # Count the occurrences of [Peer] in wg0.conf (should be 2 for a 3 node peering)
    wg_conf_peer_count=$(sudo $DOCKER exec node1 grep Peer /etc/wireguard/wg0.conf | wc -l)
    if ((wg_conf_peer_count != 2)); then
      echo "The peer count in wg0.conf  on node1 should be 2"
    fi
    wg_conf_peer_count=$(sudo $DOCKER exec node2 grep Peer /etc/wireguard/wg0.conf | wc -l)
    if ((wg_conf_peer_count != 2)); then
      echo "The peer count in wg0.conf  on node2 should be 2"
    fi
    wg_conf_peer_count=$(sudo $DOCKER exec node2 grep Peer /etc/wireguard/wg0.conf | wc -l)
    if ((wg_conf_peer_count != 2)); then
      echo "The peer count in wg0.conf on node3 should be 2"
    fi
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
setup_hub_spoke_connectivity
verify_connectivity
clean_nodes
cycle_configurations
clean_nodes

echo "e2e completed"