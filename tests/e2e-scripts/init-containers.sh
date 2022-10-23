#!/bin/bash
# fail the script if any errors are encountered
set -ex

# Globals
controller=redis
controller_passwd=floofykittens

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
    timeout 60s bash -c 'until curl -sfL http://localhost:8080/health; do sleep 1; done'

    echo "Deploy containers"
    if echo ${node_image} | grep -i fedora; then
        echo "Deploying container image ${node_image}"
         # Start Fedora Containers (requires privileged due to sysctl net.ipv4.ip_forward=1 throwing an exit 1)
        $DOCKER run -itd \
            --name=node1 \
            --net=apex_default \
            --cap-add=SYS_MODULE \
            --cap-add=NET_ADMIN \
            --cap-add=NET_RAW \
            --privileged=true \
            ${node_image}

        # Start node2post
        $DOCKER run -itd \
            --name=node2 \
            --net=apex_default \
            --cap-add=SYS_MODULE \
            --cap-add=NET_ADMIN \
            --cap-add=NET_RAW \
            --privileged=true \
            ${node_image}

        # Start node3post
        $DOCKER run -itd \
            --name=node3 \
            --net=apex_default \
            --cap-add=SYS_MODULE \
            --cap-add=NET_ADMIN \
            --cap-add=NET_RAW \
            --privileged=true \
            ${node_image}
    else
        echo "Deploying container image ${node_image}"
        # Start any other container OS type
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
    fi
}

teardown() {
    docker compose logs
    if [ -z "NO_TEARDOWN" ]; then
        return
    fi
    for node in "node1" "node2" "node3"; do
        $DOCKER kill $node || true
        $DOCKER rm $node || true
    done
    $DOCKER_COMPOSE down || true
}

copy_binaries() {
    ###########################################################################
    # Description:                                                            #
    # Copy the binaries and create the container script to start the agent    #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    # default zone id
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
APEX_LOGLEVEL=debug apex \
--public-key=${node1_pubkey} \
--private-key=${node1_pvtkey} \
--controller=${controller} \
--local-endpoint-ip=${node1_ip} \
--controller-password=${controller_passwd}
EOF

    # Node-2 apex run default zone
    cat <<EOF > apex-run-node2.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
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
    echo "=== Test: basic zone creation and connectivity ==="
    # node1 specific details
    local node1_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node1)

    # node2 specific details
    local node2_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node2)

    # Create the new zone
    local zone=$(curl -fL -X POST 'http://localhost:8080/zones' \
    -H "Authorization: bearer $API_TOKEN" \
    -H 'Content-Type: application/json' \
    --data-raw '{
        "Name": "zone-blue",
        "Description": "Tenant - Zone Blue",
        "CIDR": "10.140.0.0/20"
    }' | jq -r '.ID')

    # Node-1 apex run
    cat <<EOF > apex-run-node1.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
--controller=${controller} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Node-2 apex run
    cat <<EOF > apex-run-node2.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
--controller=${controller} \
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

    # Set permissions in the container
    $DOCKER exec node1 chmod +x /bin/apex-run-node1.sh
    $DOCKER exec node2 chmod +x /bin/apex-run-node2.sh

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/apex-run-node1.sh &
    $DOCKER exec node2 /bin/apex-run-node2.sh &

    # Allow two seconds for the wg0 interface to readdress
    sleep 2
}

setup_requested_ip_connectivity() {
    ###########################################################################
    # Description:                                                            #
    # Verify a second custom zone can be created and connected with no        #
    # errors using a different key pair as prior tests                        #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    echo "=== Test: test the request ip option ==="

    # node1 specific details
    local node1_requested_ip_cycle1=100.64.0.101
    local node1_requested_ip_cycle2=100.64.1.101
    local node1_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node1)

    # node2 specific details
    local node2_requested_ip_cycle1=100.64.0.102
    local node2_requested_ip_cycle2=100.64.1.102
    local node2_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node2)

    # Create the new zone with a CGNAT range
    local zone=$(curl -fL -X POST 'http://localhost:8080/zones' \
    -H "Authorization: bearer $API_TOKEN" \
    -H 'Content-Type: application/json' \
    --data-raw '{
        "Name": "zone-red",
        "Description": "Tenant - Zone Red",
        "CIDR": "100.64.0.0/20"
    }' | jq -r '.ID')

    # Node-1 cycle-1 apex run
    cat <<EOF > apex-cycle1-node1.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
--controller=${controller} \
--local-endpoint-ip=${node1_ip} \
--request-ip=${node1_requested_ip_cycle1} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Node-2 cycle-1 apex run
    cat <<EOF > apex-cycle1-node2.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
--controller=${controller} \
--local-endpoint-ip=${node2_ip} \
--request-ip=${node2_requested_ip_cycle1} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Node-1 cycle-2 apex run
    cat <<EOF > apex-cycle2-node1.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
--controller=${controller} \
--local-endpoint-ip=${node1_ip} \
--request-ip=${node1_requested_ip_cycle2} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Node-2 cycle-2 apex run
    cat <<EOF > apex-cycle2-node2.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
--controller=${controller} \
--local-endpoint-ip=${node2_ip} \
--request-ip=${node2_requested_ip_cycle2} \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

    # Kill the apex process on both nodes
    $DOCKER exec node1 killall apex
    $DOCKER exec node2 killall apex

    # STDOUT the run scripts for debugging
    echo "=== Displaying apex-cycle1-node1.sh ==="
    cat apex-cycle1-node1.sh
    echo "=== Displaying apex-cycle1-node2.sh ==="
    cat apex-cycle1-node2.sh

    # Delete the key pairs to force a pair regen
    $DOCKER exec node1 rm /etc/wireguard/public.key
    $DOCKER exec node1 rm /etc/wireguard/private.key
    $DOCKER exec node2 rm /etc/wireguard/public.key
    $DOCKER exec node2 rm /etc/wireguard/private.key

    # Copy files and set permissions
    $DOCKER cp ./apex-cycle1-node1.sh node1:/bin/apex-cycle1-node1.sh
    $DOCKER cp ./apex-cycle1-node2.sh node2:/bin/apex-cycle1-node2.sh
    $DOCKER exec node1 chmod +x /bin/apex-cycle1-node1.sh
    $DOCKER exec node2 chmod +x /bin/apex-cycle1-node2.sh

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/apex-cycle1-node1.sh &
    $DOCKER exec node2 /bin/apex-cycle1-node2.sh &

    # Allow five seconds for the wg0 interface to readdress
    sleep 5

    # Check connectivity between the request ip from node1 > node2
    if $DOCKER exec node1 ping -c 2 -w 2 ${node2_requested_ip_cycle1}; then
        echo "peer node updated requested ip successfully communicated"
    else
        echo "node1 failed to reach node2 updated requested ip , e2e failed"
        exit 1
    fi
    # heck connectivity between the request ip from node2 -> node1
    if $DOCKER exec node2 ping -c 2 -w 2 ${node1_requested_ip_cycle1}; then
        echo "peer node updated requested ip successfully communicated"
    else
        echo "node2 failed to reach node1 updated requested ip, e2e failed"
        exit 1
    fi

    echo "=== Test: test the requested ip got updated in the peer table and was updated on the endpoint ==="

    # Kill the apex process on both nodes
    $DOCKER exec node1 killall apex
    $DOCKER exec node2 killall apex

    # STDOUT the run scripts for debugging
    echo "=== Displaying apex-cycle2-node1.sh ==="
    cat apex-cycle2-node1.sh
    echo "=== Displaying apex-cycle2-node2.sh ==="
    cat apex-cycle2-node2.sh

    $DOCKER cp ./apex-cycle2-node1.sh node1:/bin/apex-cycle2-node1.sh
    $DOCKER cp ./apex-cycle2-node2.sh node2:/bin/apex-cycle2-node2.sh
    $DOCKER exec node1 chmod +x /bin/apex-cycle2-node1.sh
    $DOCKER exec node2 chmod +x /bin/apex-cycle2-node2.sh

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/apex-cycle2-node1.sh &
    $DOCKER exec node2 /bin/apex-cycle2-node2.sh &

    # Allow two seconds for the wg0 interface to readdress
    sleep 2

    # Check connectivity between the request ip from node1 > node2
    if $DOCKER exec node1 ping -c 2 -w 2 ${node2_requested_ip_cycle2}; then
        echo "peer node requested ip successfully communicated"
    else
        echo "node1 failed to reach node2 requested ip , e2e failed"
        exit 1
    fi
    # heck connectivity between the request ip from node2 -> node1
    if $DOCKER exec node2 ping -c 2 -w 2 ${node1_requested_ip_cycle2}; then
        echo "peer node requested ip successfully communicated"
    else
        echo "node2 failed to reach node1 requested ip, e2e failed"
        exit 1
    fi
}

setup_child_prefix_connectivity() {
    ###########################################################################
    # Description:                                                            #
    # Verify a child-prefix and request-ip can be created and add a loopback  #
    # on each node in the child prefix cidr and verify connectivity           #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    echo "=== Test: child prefix and more request ip creation and connectivity ==="

    # node1 specific details
    local requested_ip_node1=192.168.200.100
    local child_prefix_node1=172.20.1.0/24
    local node1_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node1)
    # node2 specific details
    local requested_ip_node2=192.168.200.200
    local child_prefix_node2=172.20.3.0/24
    local node2_ip=$($DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node2)

    # Create the new zone with a CGNAT range
    local zone=$(curl -fL -X POST 'http://localhost:8080/zones' \
    -H "Authorization: bearer $API_TOKEN" \
    -H 'Content-Type: application/json' \
    --data-raw '{
        "Name": "prefix-test",
        "Description": "Tenant - Zone prefix-test",
        "CIDR": "192.168.200.0/24"
    }' | jq -r '.ID')

    # Kill the apex process on both nodes
    $DOCKER exec node1 killall apex
    $DOCKER exec node2 killall apex

    # Node-1 apex run
    cat <<EOF > apex-run-node1.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --child-prefix=${child_prefix_node1} \
    --request-ip=${requested_ip_node1} \
    --zone=${zone}
EOF

    # Node-2 apex run
    cat <<EOF > apex-run-node2.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --child-prefix=${child_prefix_node2} \
    --request-ip=${requested_ip_node2} \
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

    # Set permissions in the container
    $DOCKER exec node1 chmod +x /bin/apex-run-node1.sh
    $DOCKER exec node2 chmod +x /bin/apex-run-node2.sh

    # Add loopback addresses the are in the child-prefix cidr range
    $DOCKER exec node1 ip addr add 172.20.1.10/32 dev lo
    $DOCKER exec node2 ip addr add 172.20.3.10/32 dev lo

    echo "=== Test: delete one key in the pair and to make sure the agent creates a new pair =="
    # delete one key on each node
    $DOCKER exec node1 rm /etc/wireguard/private.key
    $DOCKER exec node2 rm /etc/wireguard/public.key

    # Start the agents on both nodes
    $DOCKER exec node1 /bin/apex-run-node1.sh &
    $DOCKER exec node2 /bin/apex-run-node2.sh &

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

    # Check connectivity between the request ip from node1 > node2
    if $DOCKER exec node1 ping -c 2 -w 2 ${requested_ip_node2}; then
        echo "peer node loopbacks successfully communicated"
    else
        echo "node1 failed to reach node2 loopback, e2e failed"
        exit 1
    fi
    # heck connectivity between the request ip from node2 -> node1
    if $DOCKER exec node2 ping -c 2 -w 2 ${requested_ip_node1}; then
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

setup_hub_spoke_connectivity() {
    ###########################################################################
    # Description:                                                            #
    # Verify a child-prefix and request-ip can be created and add a loopback  #
    # on each node in the child prefix cidr and verify connectivity           #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    echo "=== Test: hub and spoke 3-node creation and connectivity ==="

    # node1 specific details
    local node1_ip=$(sudo $DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node1)
    # node2 specific details
    local node2_ip=$(sudo $DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node2)
    # node3 specific details
    local node3_ip=$(sudo $DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node3)

    # Create the new zone
    local zone=$(curl -fL -X POST 'http://localhost:8080/zones' \
    -H "Authorization: bearer $API_TOKEN" \
    -H 'Content-Type: application/json' \
    --data-raw '{
        "Name": "hub-spoke-zone",
        "Description": "Hub/Spoke Zone",
        "CIDR": "10.89.0.0/27",
        "Hub-Zone": true
    }' | jq -r '.ID')

    # Kill the apex process on both nodes (no process running on node3 yet)
    sudo $DOCKER exec node1 killall apex
    sudo $DOCKER exec node2 killall apex

    # Node-1 apex run
    cat <<EOF > apex-run-node1.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --hub-router \
    --zone=${zone}
EOF

    # Node-2 apex run
    cat <<EOF > apex-run-node2.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --zone=${zone}
EOF

    # Node-3 apex run
    cat <<EOF > apex-run-node3.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
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
    sleep 6
    verify_three_node_connectivity

    $DOCKER exec node1 killall apex
    $DOCKER exec node2 killall apex
    $DOCKER exec node3 killall apex

    echo "=== Test: Terminate the apex agents, redeploy the hub and spoke setup and test connectivity ==="
    sudo $DOCKER exec node1 /bin/apex-run-node1.sh &
    sleep 5
    sudo $DOCKER exec node2 /bin/apex-run-node2.sh &
    sudo $DOCKER exec node3 /bin/apex-run-node3.sh &
    sleep 10
    # view the wg0.conf for debugging
    sudo $DOCKER exec node1 cat /etc/wireguard/wg0.conf
    sudo $DOCKER exec node2 cat /etc/wireguard/wg0.conf
    sudo $DOCKER exec node3 cat /etc/wireguard/wg0.conf
    sleep 5
    verify_three_node_connectivity

    $DOCKER exec node1 killall apex
    $DOCKER exec node2 killall apex
    $DOCKER exec node3 killall apex
}

###########################################################################
# Description:                                                            #
# Verify 3-node connectivity. Don't use for --request-ip testing since    #
# uses whatever address is assigned to wg0                                #
###########################################################################
verify_three_node_connectivity(){
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
# This test will cycle p2p configurations to ensure database entries are  #
# work as expected and the configuration parsing works as intended        #
#                                                                         #
###########################################################################
cycle_mesh_configurations(){
    echo "=== Test: cycle configuration mesh stress tests ==="

    # node1 specific details
    local node1_ip=$(sudo $DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node1)
    # node2 specific details
    local node2_ip=$(sudo $DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node2)
    # node3 specific details
    local node3_ip=$(sudo $DOCKER inspect --format "{{ .NetworkSettings.Networks.apex_default.IPAddress }}" node3)

    # Create the new zone
    local zone=$(curl -fL -X POST 'http://localhost:8080/zones' \
    -H "Authorization: bearer $API_TOKEN" \
    -H 'Content-Type: application/json' \
    --data-raw '{
        "Name": "cycle-zone",
        "Description": "stress tester",
        "CIDR": "10.220.0.0/24"
    }' | jq -r '.ID')

    # Create configurations for three nodes
    for i in {1..3}
    do
        cat <<EOF > apex-run-node1-cycle${i}.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --local-endpoint-ip=${node1_ip} \
    --request-ip=10.220.10.${i} \
    --zone=${zone}
EOF

        cat <<EOF > apex-run-node2-cycle${i}.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --local-endpoint-ip=${node2_ip} \
    --request-ip=10.220.30.${i} \
    --zone=${zone}
EOF

        cat <<EOF > apex-run-node3-cycle${i}.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
    --controller=${controller} \
    --controller-password=${controller_passwd} \
    --local-endpoint-ip=${node3_ip} \
    --request-ip=10.220.50.${i} \
    --zone=${zone}
EOF
    done

    # Deploy the generated configurations
    for i in {1..3}
    do
        cycle_mesh_deploy ${i}
    done

    ###########################################################################
    # Description:                                                            #
    #  deploy nodes using their public NAT addresses as EndpointIPs without   #
    #  testing connectivity since it would fail in actions infra. Then revert #
    #  back to an internal address and verify connectivity                    #
    ###########################################################################

    echo "=== Test: deploy nodes using their public NAT addresses ==="
    # Node-1 apex run
    cat <<EOF > apex-pubip-node1.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
    --controller=${controller} \
    --public-network \
    --controller-password=${controller_passwd} \
    --zone=${zone}
EOF

    # Node-2 apex run
    cat <<EOF > apex-pubip-node2.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
    --controller=${controller} \
    --public-network \
    --controller-password=${controller_passwd} \
    --zone=${zone}
EOF

    # Node-3 apex run
    cat <<EOF > apex-pubip-node3.sh
#!/bin/bash
APEX_LOGLEVEL=debug apex \
    --controller=${controller} \
    --public-network \
    --controller-password=${controller_passwd} \
    --zone=${zone}
EOF

    sudo $DOCKER cp ./apex-pubip-node1.sh node1:/bin/apex-pubip-node1.sh
    sudo $DOCKER cp ./apex-pubip-node2.sh node2:/bin/apex-pubip-node2.sh
    sudo $DOCKER cp ./apex-pubip-node3.sh node3:/bin/apex-pubip-node3.sh

    # Set permissions in the container
    sudo $DOCKER exec node1 chmod +x /bin/apex-pubip-node1.sh
    sudo $DOCKER exec node2 chmod +x /bin/apex-pubip-node2.sh
    sudo $DOCKER exec node3 chmod +x /bin/apex-pubip-node3.sh

    # Start the agents on all 3 using the public NAT address as the EndpointIP
    sudo $DOCKER exec node1 /bin/apex-pubip-node1.sh &
    sudo $DOCKER exec node2 /bin/apex-pubip-node2.sh &
    sudo $DOCKER exec node3 /bin/apex-pubip-node3.sh &
    sleep 4

    # Kill processes because they are public unreachable addresses
    sudo $DOCKER exec node1 killall apex
    sudo $DOCKER exec node2 killall apex
    sudo $DOCKER exec node3 killall apex

    echo "=== Test: Redeploy the stress test cycle after using public EndpointIP addresses ==="

    # Deploy the generated configurations
    for i in {1..2}
    do
        cycle_mesh_deploy ${i}
    done
}

###########################################################################
# Description:                                                            #
# Run the following functions to test end to end connectivity between     #
# Wireguard interfaces in the container on interface wg0                  #
# Args:                                                                   #
# $1 cycle run number                                                     #
###########################################################################
cycle_mesh_deploy() {
    local cycle_count="${1}"

    # Copy files to the containers
    sudo $DOCKER cp ./apex-run-node1-cycle1.sh node1:/bin/apex-run-node1-cycle${cycle_count}.sh
    sudo $DOCKER cp ./apex-run-node2-cycle1.sh node2:/bin/apex-run-node2-cycle${cycle_count}.sh
    sudo $DOCKER cp ./apex-run-node3-cycle1.sh node3:/bin/apex-run-node3-cycle${cycle_count}.sh

    # Set permissions in the container
    sudo $DOCKER exec node1 chmod +x /bin/apex-run-node1-cycle${cycle_count}.sh
    sudo $DOCKER exec node2 chmod +x /bin/apex-run-node2-cycle${cycle_count}.sh
    sudo $DOCKER exec node3 chmod +x /bin/apex-run-node3-cycle${cycle_count}.sh

    # Start the agents on all 3 nodes nodes
    sudo $DOCKER exec node1 /bin/apex-run-node1-cycle${cycle_count}.sh &
    sudo $DOCKER exec node2 /bin/apex-run-node2-cycle${cycle_count}.sh &
    sudo $DOCKER exec node3 /bin/apex-run-node3-cycle${cycle_count}.sh &

    # Longer sleep here as Fedora has a slower wg interface convergence we will look into
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

get_token() {
    local HOST="localhost:8888"
    local REALM="controller"
    local USERNAME="admin"
    local PASSWORD="floofykittens"
    local CLIENTID='api-clients'
    local CLIENTSECRET='cvXhCRXI2Vld244jjDcnABCMrTEq2rwE'

    local token=$(curl -s -X POST \
        http://$HOST/realms/$REALM/protocol/openid-connect/token \
        -H 'Content-Type: application/x-www-form-urlencoded' \
        -d "username=$USERNAME" \
        -d "password=$PASSWORD" \
        -d "grant_type=password" \
        -d "client_id=$CLIENTID" \
        -d "client_secret=$CLIENTSECRET" | jq -r ".access_token")
    export API_TOKEN=$token
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

trap teardown EXIT
start_containers ${os}
get_token
copy_binaries
verify_connectivity
clean_nodes
setup_custom_zone_connectivity
verify_connectivity
clean_nodes
setup_requested_ip_connectivity
verify_connectivity
clean_nodes
setup_child_prefix_connectivity
verify_connectivity
clean_nodes
setup_hub_spoke_connectivity
verify_connectivity
clean_nodes
cycle_mesh_configurations
clean_nodes

echo "e2e completed"
