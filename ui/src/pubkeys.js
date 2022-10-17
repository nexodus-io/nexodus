
import { Datagrid, List, TextField, ReferenceArrayField, Show, SimpleShowLayout } from 'react-admin';

export const PubKeyList = () => (
    <List>
        <Datagrid rowClick="show">
            <TextField label = "ID" source="id" />
            <ReferenceArrayField label="Peers" source="peers" reference='peers' />
        </Datagrid>
    </List>
);

export const PubKeyShow = () => (
    <Show>
        <SimpleShowLayout>
            <TextField label="ID" source="id" />
            <ReferenceArrayField label="Peers" reference="peers" source="peers">
                <Datagrid rowClick="show" bulkActionButtons={false}>
                    <TextField label="ID" source="id" />
                    <TextField label="Public Key" source="public-key" />
                    <TextField label="IP Address" source="allowed-ips" />
                </Datagrid>
            </ReferenceArrayField>
        </SimpleShowLayout>
    </Show>
);