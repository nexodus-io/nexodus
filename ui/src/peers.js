import { Datagrid, List, Show, SimpleShowLayout, TextField } from 'react-admin';

export const PeerList = () => (
    <List>
        <Datagrid rowClick="show" bulkActionButtons={false} >
            <TextField label="ID" source="id" />
            <TextField label="Public Key" source="public-key" />
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
            <TextField label="Public Key" source="public-key" />
            <TextField label="Endpoint IP" source="endpoint-ip" />
            <TextField label="Allowed IPs" source="allowed-ips" />
            <TextField label="Zone" source="zone" />
            <TextField label="Node Address" source="node-address" />
            <TextField label="Child Prefix" source="child-prefix" />
        </SimpleShowLayout>
    </Show>
);