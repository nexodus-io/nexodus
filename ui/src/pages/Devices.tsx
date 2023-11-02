import React, { FC } from "react";
import {
  Datagrid,
  List,
  TextField,
  SimpleShowLayout,
  ReferenceField,
  BulkExportButton,
  BulkDeleteButton,
  ArrayField,
  DateField,
  BooleanField,
  Show,
  useRecordContext,
} from "react-admin";
import OnlineIcon from "@mui/icons-material/CheckCircleOutline";
import HighlightOffIcon from "@mui/icons-material/HighlightOff";
import HelpOutlineIcon from "@mui/icons-material/HelpOutline";
import { useTheme } from "@mui/material/styles";
import { Tooltip, Accordion, AccordionDetails } from "@mui/material";

interface DeviceAccordionDetailsProps {
  id: string | number;
}

const DeviceListBulkActions = () => (
  <div style={{ display: "flex", justifyContent: "space-between" }}>
    <BulkExportButton />
    <BulkDeleteButton />
  </div>
);

export const DeviceList = () => (
  <List>
    <Datagrid
      rowClick="show"
      expand={<DeviceAccordion />}
      bulkActionButtons={<DeviceListBulkActions />}
    >
      <TextField label="Hostname" source="hostname" />
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
      <ArrayField label="Endpoints" source="endpoints">
        <Datagrid rowClick="show" bulkActionButtons={false}>
          <TextField label="Address" source="address" />
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
      <BooleanField
        label="Online Status"
        source="online"
        textAlign={"center"}
        valueLabelTrue={"Connected"}
        valueLabelFalse={"Not Connected"}
        TrueIcon={OnlineIcon}
        FalseIcon={HighlightOffIcon}
      />
    </Datagrid>
  </List>
);

const DeviceAccordion: FC = () => {
  const record = useRecordContext();
  if (record && record.id !== undefined) {
    return (
      <Accordion expanded={true}>
        <DeviceAccordionDetails id={record.id} />
      </Accordion>
    );
  }
  return null;
};

const DeviceAccordionDetails: FC<DeviceAccordionDetailsProps> = ({ id }) => {
  // Use the same layout as DeviceShow
  return (
    <AccordionDetails>
      <div>
        <DeviceShowLayout />
      </div>
    </AccordionDetails>
  );
};

const ConditionalOnlineSinceField = () => {
  const record = useRecordContext();
  const theme = useTheme();

  const labelStyle = {
    fontWeight: 400,
    fontSize: "0.75rem",
    lineHeight: 1.43,
    letterSpacing: "0.00938em",
    color:
      theme.palette.mode === "light"
        ? "rgba(0, 0, 0, 0.6)"
        : "rgba(255, 255, 255, 0.7)",
    marginBottom: "0.2rem",
  };

  return record && record.online ? (
    <>
      <div style={labelStyle}>Online Since</div>
      <DateField
        source="online_at"
        options={{
          weekday: "short",
          year: "numeric",
          month: "short",
          day: "numeric",
          hour: "numeric",
          minute: "numeric",
          second: "numeric",
        }}
      />
    </>
  ) : null;
};

export const DeviceShow: FC = () => (
  <Show>
    <DeviceShowLayout />
  </Show>
);

const DeviceShowLayout: FC = () => {
  const record = useRecordContext();
  if (!record) return null;
  return (
    <SimpleShowLayout>
      <TextField label="ID" source="id" />
      <TextField label="Hostname" source="hostname" />
      <TextField label="Public Key" source="public_key" />
      <ArrayField label="v4 Tunnel IP" source="ipv4_tunnel_ips">
        <Datagrid rowClick="show" bulkActionButtons={false}>
          <TextField label="Address" source="address" />
          <TextField label="CIDR" source="cidr" />
        </Datagrid>
      </ArrayField>
      <ArrayField label="v6 Tunnel IP" source="ipv6_tunnel_ips">
        <Datagrid rowClick="show" bulkActionButtons={false}>
          <TextField label="Address" source="address" />
          <TextField label="CIDR" source="cidr" />
        </Datagrid>
      </ArrayField>      <TextField label="Organization Prefix" source="organization_prefix" />
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
      <div style={{ display: "flex", alignItems: "center" }}>
        <TextField label="Online" source="online" />
        <Tooltip
          title="Online Status displays the nexodus agent's control plane connection status to the API server."
          placement="right"
        >
          <HelpOutlineIcon fontSize="small" style={{ marginLeft: "5px" }} />
        </Tooltip>
      </div>
      <ConditionalOnlineSinceField />
    </SimpleShowLayout>
  );
};
