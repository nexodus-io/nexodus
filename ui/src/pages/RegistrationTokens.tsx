import React, { Fragment } from "react";
import {
  BooleanField,
  BooleanInput,
  BulkDeleteButton,
  BulkExportButton,
  Create,
  Datagrid,
  DateField,
  DateTimeInput,
  List,
  ReferenceField,
  ReferenceInput,
  SelectInput,
  Show,
  SimpleForm,
  SimpleShowLayout,
  TextField,
  TextInput,
  useGetIdentity,
} from "react-admin";

const RegistrationTokenListBulkActions = () => (
  <Fragment>
    <BulkExportButton />
    <BulkDeleteButton />
  </Fragment>
);

export const RegistrationTokenList = () => (
  <List>
    <Datagrid
      rowClick="show"
      bulkActionButtons={<RegistrationTokenListBulkActions />}
    >
      <TextField label="ID" source="id" />
      <ReferenceField
        label="Organization"
        source="organization_id"
        reference="organizations"
        link="show"
      />
      <BooleanField label="Single Use" source="device_id" looseValue={true} />
      <DateField label="Expiration" source="expiration" showTime={true} />
    </Datagrid>
  </List>
);

export const RegistrationTokenShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField label="ID" source="id" />
      <TextField label="Bearer Token" source="bearer_token" />
      <ReferenceField
        label="Organization"
        source="organization_id"
        reference="organizations"
        link="show"
      />
      <BooleanField label="Single Use" source="device_id" looseValue={true} />
      <ReferenceField
        label="Device"
        source="device_id"
        reference="devices"
        link="show"
      />
      <TextField label="Expiration" source="expiration" />
      <TextField label="Description" source="description" />
    </SimpleShowLayout>
  </Show>
);

export const RegistrationTokenCreate = () => {
  const { identity, isLoading, error } = useGetIdentity();
  if (isLoading || error) {
    return <div />;
  }
  return (
    <Create>
      <SimpleForm>
        <TextInput
          label="Description"
          name="description"
          source="description"
          fullWidth
        />
        <ReferenceInput
          label="Organization"
          name="organization_id"
          source="organization_id"
          reference="organizations"
          filter={{ owner_id: identity.id }}
        />
        <BooleanInput
          label="Single Use"
          name="single_use"
          source="single_use"
        />
        <DateTimeInput
          label="Expiration"
          name="expiration"
          source="expiration"
        />
      </SimpleForm>
    </Create>
  );
};
