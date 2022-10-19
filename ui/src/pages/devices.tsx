
import { Datagrid, List, TextField, ReferenceArrayField, Show, SimpleShowLayout, SingleFieldList, ChipField } from 'react-admin';

export const DeviceList = () => (
    <List>
        <Datagrid rowClick="show" bulkActionButtons={false}>
            <TextField label = "ID" source="id" />
            <ReferenceArrayField label="Peers" source="peer-ids" reference="peers">
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
            <ReferenceArrayField label="Peers" source="peer-ids" reference="peers">
                <Datagrid rowClick="show" bulkActionButtons={false}>
                    <TextField label="ID" source="id" />
                    <TextField label="IP Address" source="allowed-ips" />
                </Datagrid>
            </ReferenceArrayField>
        </SimpleShowLayout>
    </Show>
);