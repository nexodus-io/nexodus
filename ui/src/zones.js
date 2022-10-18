import { Datagrid, List, TextField, SimpleForm, TextInput, ReferenceField, ReferenceArrayField, Create, Show, SimpleShowLayout } from 'react-admin';

export const ZoneList = () => (
    <List>
        <Datagrid rowClick="show" bulkActionButtons={false}>
            <TextField label="ID" source="id" />
            <TextField label="Name" source="name" />
            <TextField label="Description" source="description" />
            <TextField label="CIDR" source="cidr" />
        </Datagrid>
    </List>
)

export const ZoneShow = () => (
    <Show>
        <SimpleShowLayout>
            <TextField label="ID" source="id" />
            <TextField label="Name" source="name" />
            <TextField label="Description" source="description" />
            <TextField label="CIDR" source="cidr" />
            <ReferenceArrayField label="Peers" source="peer-ids" reference="peers">
                <Datagrid rowClick="show" bulkActionButtons={false}>
                    <TextField label="ID" source="id" />
                    <ReferenceField label="Device" source="device-id" reference="devices" />
                    <TextField label="IP Address" source="allowed-ips" />
                </Datagrid>
            </ReferenceArrayField>
        </SimpleShowLayout>
    </Show>
)

export const ZoneCreate = () => (
    <Create>
        <SimpleForm>
            <TextInput label="Name" source="name" />
            <TextInput label="Description" source="description" />
            <TextInput label="CIDR" source="cidr" />
        </SimpleForm>
    </Create>
);