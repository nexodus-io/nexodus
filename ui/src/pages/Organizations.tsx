import { Fragment } from "react";
import {
  Datagrid,
  List,
  TextField,
  ReferenceArrayField,
  Show,
  SimpleShowLayout,
  SingleFieldList,
  ReferenceField,
  useRecordContext,
  useGetOne,
  Loading,
  BulkExportButton,
  BulkDeleteButton,
  ReferenceManyField,
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
