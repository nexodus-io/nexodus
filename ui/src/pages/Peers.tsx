import { Datagrid, List, ReferenceField, Show, SimpleShowLayout, TextField, ReferenceInput } from 'react-admin';

const peerFilters =  [
    <ReferenceInput source="device_id" label="Device" reference="devices" />,
]

export const PeerList = () => (
    <List filters={peerFilters}>
        <Datagrid rowClick="show" bulkActionButtons={false} >
            <TextField label="ID" source="id" />
            <ReferenceField label="Device" source="device_id" reference='devices' link="show"/>
            <TextField label="Endpoint IP" source="endpoint_ip" />
            <TextField label="Allowed IPs" source="allowed_ips" />
            <ReferenceField label="Zone ID" source="zone_id" reference='zones' link="show" />
            <TextField label="Node Address" source="node_address" />
            <TextField label="STUN Address" source="reflexive_ip4" />
            <TextField label="Child Prefix" source="child_prefix" />
        </Datagrid>
    </List>
);

export const PeerShow= () => (
    <Show>
        <SimpleShowLayout>
            <TextField label="ID" source="id" />
            <ReferenceField label="Device" source="device_id" reference='devices' link="show" />
            <TextField label="Endpoint IP" source="endpoint_ip" />
            <TextField label="Allowed IPs" source="allowed_ips" />
            <ReferenceField label="Zone ID" source="zone_id" reference='zones' link="show" />
            <TextField label="Node Address" source="node_address" />
            <TextField label="STUN Address" source="reflexive_ip4" />
            <TextField label="Child Prefix" source="child_prefix" />
        </SimpleShowLayout>
    </Show>
);
