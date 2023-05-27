import { Fragment } from "react";
import {
  Datagrid,
  List,
  TextField,
  Show,
  SimpleShowLayout,
  ReferenceField,
  BulkExportButton,
  BulkDeleteButton,
  ReferenceManyField,
  Create,
  SimpleForm,
  TextInput,
} from "react-admin";

const OrganizationListBulkActions = () => (
  <Fragment>
    <BulkExportButton />
    <BulkDeleteButton />
  </Fragment>
);

export const OrganizationList = () => (
  <List>
    <Datagrid
      rowClick="show"
      bulkActionButtons={<OrganizationListBulkActions />}
    >
      <TextField label="Name" source="name" />
      <TextField label="Description" source="description" />
      <TextField label="CIDR" source="cidr" />
      <TextField label="Relay" source="hub_zone" />
      <ReferenceField
        label="Owner"
        source="owner_id"
        reference="users"
        link="show"
      />
    </Datagrid>
  </List>
);

export const OrganizationShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField label="ID" source="id" />
      <TextField label="Name" source="name" />
      <TextField label="Description" source="description" />
      <TextField label="Organization CIDR" source="cidr" />
      <TextField label="Relay Enabled" source="hub_zone" />

      <ReferenceManyField
        label="Enrolled Devices"
        reference="devices"
        target="organization_id"
      >
        <Datagrid>
          <TextField label="Hostname" source="hostname" />
          <TextField label="Tunnel IP" source="tunnel_ip" />
        </Datagrid>
      </ReferenceManyField>
    </SimpleShowLayout>
  </Show>
);

export const OrganizationCreate = () => (
  <Create>
    <SimpleForm>
      <TextInput label="Name" source="name" />
      <TextInput label="Description" source="description" />
      <TextInput label="CIDR" source="cidr" />
      <TextInput label="CIDR v6" source="cidr_v6" />
    </SimpleForm>
  </Create>
);
