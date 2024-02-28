import React, { Fragment } from "react";
import {
  ArrayField,
  AutocompleteInput,
  BulkDeleteButton,
  BulkExportButton,
  Create,
  Datagrid,
  List,
  ReferenceField,
  ReferenceInput,
  ReferenceManyCount,
  ReferenceManyField,
  Show,
  SimpleForm,
  SimpleShowLayout,
  TextField,
  TextInput,
} from "react-admin";
import { useFlags } from "../common/FlagsContext";

const VPCListBulkActions = () => (
  <Fragment>
    <BulkExportButton />
    <BulkDeleteButton />
  </Fragment>
);

export const VPCList = () => {
  const flags = useFlags();
  return (
    <List>
      <Datagrid rowClick="show" bulkActionButtons={<VPCListBulkActions />}>
        <TextField label="Description" source="description" />
        {flags["devices"] && (
          <div style={{ display: "flex", alignItems: "center" }}>
            <div style={{ marginRight: "8px" }}>
              <TextField label="v4 CIDR" source="ipv4_cidr" />
            </div>
            <TextField label="v6 CIDR" source="ipv6_cidr" />
            <ReferenceManyCount
              label="Devices"
              reference="devices"
              target="vpc_id"
            />
          </div>
        )}
        <ReferenceField
          label="Organization"
          source="organization_id"
          reference="organizations"
          link="show"
        />
      </Datagrid>
    </List>
  );
};

export const VPCShow = () => {
  const flags = useFlags();
  return (
    <Show>
      <SimpleShowLayout>
        <TextField label="ID" source="id" />
        <TextField label="Description" source="description" />
        {flags["devices"] && (
          <div style={{ display: "block", marginBottom: "1rem" }}>
            <div
              style={{
                display: "flex",
                alignItems: "center",
                marginBottom: "8px",
              }}
            >
              <TextField
                label="v4 CIDR"
                source="ipv4_cidr"
                style={{ marginRight: "8px" }}
              />
              <TextField label="v6 CIDR" source="ipv6_cidr" />
            </div>
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
          </div>
        )}
        <ReferenceField
          label="Organization"
          source="organization_id"
          reference="organizations"
          link="show"
        />
      </SimpleShowLayout>
    </Show>
  );
};

export const VPCCreate = () => {
  const flags = useFlags();
  return (
    <Create>
      <SimpleForm>
        <TextInput label="Name" source="name" fullWidth />
        <TextInput label="Description" source="description" fullWidth />
        {flags["devices"] && (
          <div>
            <TextInput label="CIDR v4" source="ipv4_cidr" fullWidth />
            <TextInput label="CIDR v6" source="ipv6_cidr" fullWidth />
          </div>
        )}
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
};
