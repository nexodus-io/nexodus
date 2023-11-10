import React, { Fragment } from "react";
import {
  AutocompleteInput,
  BooleanField,
  BooleanInput,
  BulkDeleteButton,
  BulkExportButton,
  Create,
  Datagrid,
  DateField,
  DateTimeInput,
  Edit,
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
      <TextField label="Description" source="description" />
      <ReferenceField label="VPC" source="vpc_id" reference="vpcs" />
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
      <ReferenceField
        label="Security Group"
        source="security_group_id"
        reference="security-groups"
        // We can't deep link to security groups yet...
        // link={(record) =>{
        //   return `/_security-groups/${record.id}`
        // }}
      />
      <ReferenceField
        label="Device"
        source="device_id"
        reference="devices"
        link="show"
      />
      <BooleanField label="Single Use" source="device_id" looseValue={true} />
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
        <ReferenceInput name="vpc_id" source="vpc_id" reference="vpcs">
          <AutocompleteInput fullWidth />
        </ReferenceInput>
        <ReferenceInput
          name="security_group_id"
          source="security_group_id"
          reference="security-groups"
        >
          <AutocompleteInput fullWidth />
        </ReferenceInput>
        <BooleanInput
          label="Single Use"
          name="single_use"
          source="single_use"
          fullWidth
        />
        <DateTimeInput
          label="Expiration"
          name="expiration"
          source="expiration"
          fullWidth
        />
      </SimpleForm>
    </Create>
  );
};

export const RegKeyEdit = () => {
  const { identity, isLoading, error } = useGetIdentity();
  if (isLoading || error) {
    return <div />;
  }
  return (
    <Edit>
      <SimpleForm>
        <TextInput
          label="Description"
          name="description"
          source="description"
          fullWidth
        />
        <ReferenceInput
          name="security_group_id"
          source="security_group_id"
          reference="security-groups"
        >
          <AutocompleteInput fullWidth />
        </ReferenceInput>
        <BooleanInput
          label="Single Use"
          name="single_use"
          source="single_use"
          fullWidth
        />
        <DateTimeInput
          label="Expiration"
          name="expiration"
          source="expiration"
          fullWidth
        />
      </SimpleForm>
    </Edit>
  );
};
