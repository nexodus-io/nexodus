import {
    Datagrid,
    List,
    TextField,
    SimpleForm,
    TextInput,
    ReferenceField,
    ReferenceArrayField,
    Create,
    Show,
    SimpleShowLayout,
    BooleanField,
} from 'react-admin';

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
            <BooleanField label="Hub Zone" source="hub-zone" />
            <ReferenceArrayField label="Peers" source="peers" reference="peers">
                <Datagrid rowClick="show" bulkActionButtons={false} >
                    <ReferenceField label="ID" source="id" reference="peers" link="show"/>
                    <ReferenceField label="Device" source="device-id" reference='devices' link="show"/>
                    <TextField label="Endpoint IP" source="endpoint-ip" />
                    <TextField label="Node Address" source="node-address" />
                    <BooleanField label="Hub Router" source="hub-router" />
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
