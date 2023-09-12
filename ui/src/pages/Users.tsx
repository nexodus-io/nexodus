import { Fragment } from "react";
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
  BulkExportButton,
  BulkDeleteButton,
  ReferenceManyField,
} from "react-admin";

const UserListBulkActions = () => (
  <Fragment>
    <BulkExportButton />
  </Fragment>
);

export const UserList = () => (
  <List>
    <Datagrid rowClick="show" bulkActionButtons={<UserListBulkActions />}>
      <TextField label="Username" source="username" />
    </Datagrid>
  </List>
);

export const UserShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField label="ID" source="id" />
      <TextField label="Username" source="username" />

      <ReferenceManyField
        label="Owned Organizations"
        reference="organizations"
        target="owner_id"
      >
        <Datagrid>
          <TextField label="Name" source="name" />
          <TextField label="Description" source="description" />
        </Datagrid>
      </ReferenceManyField>

      <ReferenceManyField
        label="Owned Devices"
        reference="devices"
        target="user_id"
      >
        <Datagrid>
          <TextField label="Hostname" source="hostname" />
          <TextField label="Tunnel IP" source="tunnel_ip" />
        </Datagrid>
      </ReferenceManyField>
    </SimpleShowLayout>
  </Show>
);
