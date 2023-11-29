import React, { FC, forwardRef } from "react";
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
  BooleanFieldProps,
  Show,
  useRecordContext,
  useGetIdentity,
  Edit,
  SimpleForm,
  TextInput,
  ReferenceInput,
  AutocompleteInput,
  BooleanInput,
  DateTimeInput,
} from "react-admin";
import OnlineIcon from "@mui/icons-material/CheckCircleOutline";
import HighlightOffIcon from "@mui/icons-material/HighlightOff";
import HelpOutlineIcon from "@mui/icons-material/HelpOutline";
import { useTheme } from "@mui/material/styles";
import { Tooltip, Accordion, AccordionDetails, SvgIcon } from "@mui/material";

interface DeviceAccordionDetailsProps {
  id: string | number;
}

const DeviceListBulkActions = () => (
  <div style={{ display: "flex", justifyContent: "space-between" }}>
    <BulkExportButton />
    <BulkDeleteButton />
  </div>
);

const StatusBooleanField = (props: BooleanFieldProps): JSX.Element => {
  const { source, label, valueLabelTrue } = props;
  const record = useRecordContext(props);

  const TrueIcon = forwardRef<any, any>((props, ref) => {
    return <OnlineIcon color="success" ref={ref} {...props} />;
  }) as unknown as typeof SvgIcon;
  TrueIcon.muiName = "TrueIcon";

  const FalseIcon = forwardRef<any, any>((props, ref) => {
    return <HighlightOffIcon color="disabled" ref={ref} {...props} />;
  }) as unknown as typeof SvgIcon;
  FalseIcon.muiName = "FalseIcon";

  return (
    <BooleanField
      label="Online Status"
      source="online"
      textAlign={"center"}
      valueLabelTrue={"Connected"}
      valueLabelFalse={"Not Connected"}
      TrueIcon={TrueIcon}
      FalseIcon={FalseIcon}
    />
  );
};

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
        label="VPC"
        source="vpc_id"
        reference="vpcs"
        link="show"
      />
      <ReferenceField
        label="Owner"
        source="owner_id"
        reference="users"
        link="show"
      />
      <StatusBooleanField label="Online Status" />
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
      </ArrayField>
      <TextField label="Allowed IPs" source="allowed_ips" />
      <ArrayField label="Endpoints" source="endpoints">
        <Datagrid rowClick="show" bulkActionButtons={false}>
          <TextField label="Address" source="address" />
          <TextField label="Source" source="source" />
        </Datagrid>
      </ArrayField>
      <TextField label="Relay Node" source="relay" />
      <ReferenceField
        label="VPC"
        source="vpc_id"
        reference="vpcs"
        link="show"
      />
      <ReferenceField
        label="Owner"
        source="owner_id"
        reference="users"
        link="show"
      />
      <StatusBooleanField label="Online Status" />
      <ConditionalOnlineSinceField />
    </SimpleShowLayout>
  );
};

export const DeviceEdit = () => {
  const { identity, isLoading, error } = useGetIdentity();
  if (isLoading || error) {
    return <div />;
  }
  return (
    <Edit>
      <SimpleForm>
        <TextInput
          label="Hostname"
          name="hostname"
          source="hostname"
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
      </SimpleForm>
    </Edit>
  );
};
