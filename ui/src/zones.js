import { Datagrid, List, TextField, SimpleForm, TextInput, Create, Show, SimpleShowLayout } from 'react-admin';

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