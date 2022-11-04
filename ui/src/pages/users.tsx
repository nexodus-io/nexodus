
import { 
    Datagrid,
    List,
    TextField,
    ReferenceArrayField,
    Show,
    SimpleShowLayout,
    SingleFieldList,
    ChipField
} from 'react-admin';

export const UserList = () => (
    <List>
        <Datagrid rowClick="show" bulkActionButtons={false}>
            <TextField label = "ID" source="id" />
            <TextField label = "Zone ID" source="zone-id" />
            <ReferenceArrayField label="Devices" source="devices" reference="devices">
                <SingleFieldList linkType="show">
                    <ChipField source="id" />
                </SingleFieldList>
            </ReferenceArrayField>
        </Datagrid>
    </List>
);

export const UserShow = () => (
    <Show>
        <SimpleShowLayout>
            <TextField label = "ID" source="id" />
            <TextField label = "Zone ID" source="zone-id" />
            <ReferenceArrayField label="Devices" source="devices" reference="devices">
                <Datagrid rowClick="show" bulkActionButtons={false}>
                    <TextField label="ID" source="id" />
                </Datagrid>
            </ReferenceArrayField>
        </SimpleShowLayout>
    </Show>
);
