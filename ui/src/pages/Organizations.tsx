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
      <TextField label="ID" source="id" />
      <TextField label="Name" source="name" />
      <TextField label="Description" source="description" />
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
    </SimpleShowLayout>
  </Show>
);
