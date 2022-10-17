import { Datagrid, List, ReferenceField, Show, SimpleShowLayout, TextField, ReferenceInput } from 'react-admin';

const peerFilters =  [
    <ReferenceInput source="public-key" label="Public Key" reference="pubkeys" />,,
]

export const PeerList = () => (
    <List filters={peerFilters}>
        <Datagrid rowClick="show" bulkActionButtons={false} >
            <TextField label="ID" source="id" />
            <ReferenceField label="Public Key" source="public-key" reference='pubkeys' />
            <TextField label="Endpoint IP" source="endpoint-ip" />
            <TextField label="Allowed IPs" source="allowed-ips" />
            <TextField label="Zone" source="zone" />
            <TextField label="Node Address" source="node-address" />
            <TextField label="Child Prefix" source="child-prefix" />
        </Datagrid>
    </List>
);

export const PeerShow= () => (
    <Show>
        <SimpleShowLayout>
            <TextField label="ID" source="id" />
            <ReferenceField label="Public Key" source="public-key" reference='pubkeys' />
            <TextField label="Endpoint IP" source="endpoint-ip" />
            <TextField label="Allowed IPs" source="allowed-ips" />
            <TextField label="Zone" source="zone" />
            <TextField label="Node Address" source="node-address" />
            <TextField label="Child Prefix" source="child-prefix" />
        </SimpleShowLayout>
    </Show>
);