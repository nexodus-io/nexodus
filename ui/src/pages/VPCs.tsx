import React, { Fragment } from "react";
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
  ArrayField,
  ReferenceManyCount,
  ReferenceInput,
  AutocompleteInput,
} from "react-admin";

const VPCListBulkActions = () => (
  <Fragment>
    <BulkExportButton />
    <BulkDeleteButton />
  </Fragment>
);

export const VPCList = () => (
  <List>
    <Datagrid rowClick="show" bulkActionButtons={<VPCListBulkActions />}>
      <TextField label="Description" source="description" />
      <TextField label="v4 CIDR" source="ipv4_cidr" />
      <TextField label="v6 CIDR" source="ipv6_cidr" />
      <ReferenceManyCount label="Devices" reference="devices" target="vpc_id" />
      <ReferenceField
        label="Organization"
        source="organization_id"
        reference="organizations"
        link="show"
      />
    </Datagrid>
  </List>
);

export const VPCShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField label="ID" source="id" />
      <TextField label="Description" source="description" />
      <TextField label="v4 CIDR" source="ipv4_cidr" />
      <TextField label="v6 CIDR" source="ipv6_cidr" />

      <ReferenceManyField
        label="Enrolled Devices"
        reference="devices"
        target="vpc_id"
      >
        <Datagrid>
          <TextField label="Hostname" source="hostname" />
          <TextField label="Tunnel IP" source="tunnel_ip" />
          <ArrayField label="v4 Tunnel IP" source="ipv4_tunnel_ips">
            <Datagrid rowClick="show" bulkActionButtons={false}>
              <TextField label="Address" source="address" />
            </Datagrid>
          </ArrayField>
          <ArrayField label="v6 Tunnel IP" source="ipv6_tunnel_ips">
            <Datagrid rowClick="show" bulkActionButtons={false}>
              <TextField label="Address" source="address" />
            </Datagrid>
          </ArrayField>
        </Datagrid>
      </ReferenceManyField>
    </SimpleShowLayout>
  </Show>
);

export const VPCCreate = () => (
  <Create>
    <SimpleForm>
      <TextInput label="Name" source="name" fullWidth />
      <TextInput label="Description" source="description" fullWidth />
      <TextInput label="CIDR v4" source="ipv4_cidr" fullWidth />
      <TextInput label="CIDR v6" source="ipv6_cidr" fullWidth />
      <ReferenceInput
        name="organization_id"
        source="organization_id"
        reference="organizations"
      >
        <AutocompleteInput fullWidth />
      </ReferenceInput>
    </SimpleForm>
  </Create>
);
