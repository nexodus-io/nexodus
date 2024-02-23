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
import { UserOrganizationList } from "./UserOrganizations";

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
    </Datagrid>
  </List>
);

export const OrganizationShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField label="ID" source="id" />
      <TextField label="Name" source="name" />
      <TextField label="Description" source="description" />
      <h2>User Membership</h2>
      <UserOrganizationList />
    </SimpleShowLayout>
  </Show>
);

export const OrganizationCreate = () => (
  <Create>
    <SimpleForm>
      <TextInput label="Name" source="name" />
      <TextInput label="Description" source="description" />
    </SimpleForm>
  </Create>
);
