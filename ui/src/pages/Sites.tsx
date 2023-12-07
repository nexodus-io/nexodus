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
import { Tooltip, Accordion, AccordionDetails } from "@mui/material";

interface SiteAccordionDetailsProps {
  id: string | number;
}

const SiteListBulkActions = () => (
  <div style={{ display: "flex", justifyContent: "space-between" }}>
    <BulkExportButton />
    <BulkDeleteButton />
  </div>
);

export const SiteList = () => (
  <List>
    <Datagrid
      rowClick="show"
      expand={<SiteAccordion />}
      bulkActionButtons={<SiteListBulkActions />}
    >
      <TextField label="Name" source="name" />
      <TextField label="Platform" source="platform" />
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
    </Datagrid>
  </List>
);

const SiteAccordion: FC = () => {
  const record = useRecordContext();
  if (record && record.id !== undefined) {
    return (
      <Accordion expanded={true}>
        <SiteAccordionDetails id={record.id} />
      </Accordion>
    );
  }
  return null;
};

const SiteAccordionDetails: FC<SiteAccordionDetailsProps> = ({ id }) => {
  // Use the same layout as SiteShow
  return (
    <AccordionDetails>
      <div>
        <SiteShowLayout />
      </div>
    </AccordionDetails>
  );
};

export const SiteShow: FC = () => (
  <Show>
    <SiteShowLayout />
  </Show>
);

const SiteShowLayout: FC = () => {
  const record = useRecordContext();
  if (!record) return null;
  return (
    <SimpleShowLayout>
      <TextField label="ID" source="id" />
      <TextField label="Name" source="name" />
      <TextField label="Platform" source="platform" />
      <TextField label="Hostname" source="hostname" />
      <TextField label="OS" source="os" />
      <TextField label="Public Key" source="public_key" />
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
    </SimpleShowLayout>
  );
};

export const SiteEdit = () => {
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
      </SimpleForm>
    </Edit>
  );
};
