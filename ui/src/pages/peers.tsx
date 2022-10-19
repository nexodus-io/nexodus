import { Datagrid, List, ReferenceField, Show, SimpleShowLayout, TextField, ReferenceInput } from 'react-admin';

const peerFilters =  [
    <ReferenceInput source="device-id" label="Device" reference="devices" />,
]

export const PeerList = () => (
    <List filters={peerFilters}>
        <Datagrid rowClick="show" bulkActionButtons={false} >
            <TextField label="ID" source="id" />
            <ReferenceField label="Device" source="device-id" reference='devices' link="show"/>
            <TextField label="Endpoint IP" source="endpoint-ip" />
            <TextField label="Allowed IPs" source="allowed-ips" />
            <ReferenceField label="Zone ID" source="zone-id" reference='zones' link="show" />
            <TextField label="Node Address" source="node-address" />
            <TextField label="Child Prefix" source="child-prefix" />
        </Datagrid>
    </List>
);

export const PeerShow= () => (
    <Show>
        <SimpleShowLayout>
            <TextField label="ID" source="id" />
            <ReferenceField label="Device" source="device-id" reference='devices' />
            <TextField label="Endpoint IP" source="endpoint-ip" />
            <TextField label="Allowed IPs" source="allowed-ips" />
            <ReferenceField label="Zone ID" source="zone-id" reference='zones' link="show" />
            <TextField label="Node Address" source="node-address" />
            <TextField label="Child Prefix" source="child-prefix" />
        </SimpleShowLayout>
    </Show>
);