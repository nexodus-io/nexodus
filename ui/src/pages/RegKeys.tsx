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

const RegKeyListBulkActions = () => (
  <Fragment>
    <BulkExportButton />
    <BulkDeleteButton />
  </Fragment>
);

export const RegKeyList = () => (
  <List>
    <Datagrid rowClick="show" bulkActionButtons={<RegKeyListBulkActions />}>
      <TextField label="ID" source="id" />
      <ReferenceField
        label="VPC"
        source="vpc_id"
        reference="vpcs"
        link="show"
      />
      <BooleanField label="Single Use" source="device_id" looseValue={true} />
      <DateField label="Expiration" source="expiration" showTime={true} />
    </Datagrid>
  </List>
);

export const RegKeyShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField label="ID" source="id" />
      <TextField label="Bearer Token" source="bearer_token" />
      <ReferenceField
        label="Organization"
        source="vpc_id"
        reference="vpcs"
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

export const RegKeyCreate = () => {
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
        <ReferenceInput name="vpc_id" source="vpc_id" reference="vpcs" />
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
