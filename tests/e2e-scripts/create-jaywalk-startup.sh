#!/bin/bash
# Script to generate the jaywalk run on a CI container with the following args

pubkey=${1}
pvtkey=${2}
redis=${3}
local_endpoint=${4}
controller_passwd=${5}
zone=${6}
script_output_name=${7}

echo "Public Key: ${pubkey}"
echo "Private Key: ${pvtkey}"
echo "Redis Instance: ${redis}"
echo "Local Endpoint IP: ${local_endpoint}"
echo "Controller Password ${controller_passwd}"
echo "Zone Name: ${zone}"
echo "Script Name: ${script_output_name}"

# Create the script
cat << EOF > ${7}
#!/bin/bash

jaywalk \
--public-key=${pubkey} \
--private-key=${pvtkey}  \
--controller=${redis}  \
--local-endpoint-ip=${local_endpoint} \
--agent-mode \
--zone=${zone} \
--controller-password=${controller_passwd}
EOF

chmod +x ${script_output_name}