
import {
    Datagrid,
    List,
    TextField,
    ReferenceArrayField,
    Show,
    SimpleShowLayout,
    SingleFieldList,
    ChipField,
    ReferenceField,
} from 'react-admin';

export const DeviceList = () => (
    <List>
        <Datagrid rowClick="show" bulkActionButtons={false}>
            <TextField label = "Hostname" source="hostname" />
            <ReferenceField label = "User" source="user_id" reference="users" link="show"/>
            <TextField label = "Public Key" source="public_key" />
            <ReferenceArrayField label="Peers" source="peers" reference="peers">
                <SingleFieldList linkType="show">
                    <ChipField source="id" />
                </SingleFieldList>
            </ReferenceArrayField>
        </Datagrid>
    </List>
);

export const DeviceShow = () => (
    <Show>
        <SimpleShowLayout>
            <TextField label="ID" source="id" />
            <TextField label = "Hostname" source="hostname" />
            <ReferenceField label = "User" source="user_id" reference="users" link="show"/>
            <TextField label = "Public Key" source="public_key" />
            <ReferenceArrayField label="Peers" source="peers" reference="peers">
                <Datagrid rowClick="show" bulkActionButtons={false}>
                    <TextField label="ID" source="id" />
                    <TextField label="IP Address" source="allowed_ips" />
                </Datagrid>
            </ReferenceArrayField>
        </SimpleShowLayout>
    </Show>
);
