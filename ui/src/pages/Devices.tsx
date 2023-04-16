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
  ArrayField,
} from "react-admin";

const ZoneNameFromPeer = () => {
  const peer = useRecordContext();
  if (!peer) return null;
  const {
    data: zone,
    isLoading,
    error,
  } = useGetOne("zones", { id: peer.zone_id });
  if (isLoading) {
    return <Loading />;
  }
  if (error) {
    return <p>ERROR</p>;
  }
  return <div>{zone.name}</div>;
};

const DeviceListBulkActions = () => (
  <Fragment>
    <BulkExportButton />
    <BulkDeleteButton />
  </Fragment>
);

export const DeviceList = () => (
  <List>
    <Datagrid rowClick="show" bulkActionButtons={<DeviceListBulkActions />}>
      <TextField label="Hostname" source="hostname" />
      <TextField label="Tunnel IP" source="tunnel_ip" />

      <ArrayField label="Endpoints" source="endpoints">
        <Datagrid rowClick="show" bulkActionButtons={false}>
          <TextField label="Address" source="address" />
          <TextField label="Distance" source="distance" />
        </Datagrid>
      </ArrayField>
      <ReferenceField
        label="Organization"
        source="organization_id"
        reference="organizations"
        link="show"
      />
      <ReferenceField
        label="Owner"
        source="user_id"
        reference="users"
        link="show"
      />
    </Datagrid>
  </List>
);

export const DeviceShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField label="ID" source="id" />
      <TextField label="Hostname" source="hostname" />
      <TextField label="Public Key" source="public_key" />
      <TextField label="Tunnel IP" source="tunnel_ip" />
      <TextField label="Organization Prefix" source="organization_prefix" />
      <TextField label="Allowed IPs" source="allowed_ips" />
      <ArrayField label="Endpoints" source="endpoints">
        <Datagrid rowClick="show" bulkActionButtons={false}>
          <TextField label="Address" source="address" />
          <TextField label="Distance" source="distance" />
          <TextField label="Source" source="source" />
        </Datagrid>
      </ArrayField>
      <TextField label="Relay Node" source="relay" />
      <ReferenceField
        label="Organization"
        source="organization_id"
        reference="organizations"
        link="show"
      />
      <ReferenceField
        label="Owner"
        source="user_id"
        reference="users"
        link="show"
      />
    </SimpleShowLayout>
  </Show>
);
