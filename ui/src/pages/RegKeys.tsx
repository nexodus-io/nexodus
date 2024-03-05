import React, { Fragment, useState } from "react";
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
  Identifier,
  List,
  RaRecord,
  ReferenceField,
  ReferenceInput,
  Show,
  SimpleForm,
  SimpleShowLayout,
  TabbedForm,
  TabbedFormView,
  TextField,
  TextInput,
  useGetIdentity,
  useRecordContext,
  UseRecordContextParams,
  WithRecord,
} from "react-admin";

// @ts-ignore
import { JsonInput } from "./JsonInput";
// @ts-ignore
import { JsonField } from "./JsonField";
import { useFlags } from "../common/FlagsContext";
import {
  Card,
  CardContent,
  CardHeader,
  Checkbox,
  FormControlLabel,
  Paper,
  Tab,
  Tabs,
  Typography,
} from "@mui/material";

const RegKeyListBulkActions = () => (
  <Fragment>
    <BulkExportButton />
    <BulkDeleteButton />
  </Fragment>
);

export const RegKeyList = () => {
  const flags = useFlags();

  return (
    <List>
      <Datagrid rowClick="show" bulkActionButtons={<RegKeyListBulkActions />}>
        <TextField label="Description" source="description" />
        {flags["devices"] && (
          <ReferenceField label="VPC" source="vpc_id" reference="vpcs" />
        )}
        {flags["sites"] && (
          <ReferenceField
            label="Service Network"
            source="service_network_id"
            reference="service-networks"
          />
        )}
        <BooleanField label="Single Use" source="device_id" looseValue={true} />
        <DateField label="Expiration" source="expiration" showTime={true} />
      </Datagrid>
    </List>
  );
};

export const RegKeyFlagField = (
  props: UseRecordContextParams<RaRecord<Identifier>> | undefined,
) => {
  const record = useRecordContext(props);
  return record ? (
    <>
      <pre>
        {props?.prefix} --reg-key '{window.location.origin}#
        {record.bearer_token}'
      </pre>
    </>
  ) : null;
};

export const RegKeyShow = () => {
  const flags = useFlags();

  return (
    <Show>
      <SimpleShowLayout>
        <TextField label="ID" source="id" />
        <TextField label="Bearer Token" source="bearer_token" />
        <TextField label="Description" source="description" />
        <BooleanField label="Single Use" source="device_id" looseValue={true} />
        <DateField label="Expiration" source="expiration" showTime={true} />
        <WithRecord
          render={(record) => (
            <>
              {flags["devices"] && record.vpc_id != undefined && (
                <Paper sx={{ width: "100%", marginTop: "1em" }}>
                  <Card>
                    <CardContent>
                      <Typography gutterBottom variant="h5" component="div">
                        Allows Device Registration
                      </Typography>

                      <SimpleShowLayout>
                        <RegKeyFlagField
                          label="Command Line Flag"
                          prefix="nexd"
                        />

                        <ReferenceField
                          source="vpc_id"
                          reference="vpcs"
                          link="show"
                        />

                        <ReferenceField
                          label="Security Group"
                          source="security_group_id"
                          reference="security-groups"
                        />

                        {record.device_id && (
                          <ReferenceField
                            label="Device"
                            source="device_id"
                            reference="devices"
                            link="show"
                          />
                        )}
                      </SimpleShowLayout>
                    </CardContent>
                  </Card>
                </Paper>
              )}

              {flags["sites"] && record.service_network_id != undefined && (
                <Paper sx={{ width: "100%", marginTop: "1em" }}>
                  <Card>
                    <CardContent>
                      <Typography gutterBottom variant="h5" component="div">
                        Allows Site Registration
                      </Typography>

                      <SimpleShowLayout>
                        <RegKeyFlagField
                          label="Command Line Flag"
                          prefix="skupper init"
                        />

                        <ReferenceField
                          label="Service Network"
                          source="service_network_id"
                          reference="service-networks"
                          link="show"
                        />

                        {record.device_id && (
                          <ReferenceField
                            label="Site"
                            source="device_id"
                            reference="sites"
                            link="show"
                          />
                        )}
                      </SimpleShowLayout>
                    </CardContent>
                  </Card>
                </Paper>
              )}
            </>
          )}
        />

        {/*
        <JsonField label="Settings" source="settings" />
        */}
      </SimpleShowLayout>
    </Show>
  );
};

export const RegKeyCreate = () => {
  const flags = useFlags();
  const { identity, isLoading, error } = useGetIdentity();
  const [allowDevices, setAllowDevices] = useState(false);
  const [allowSites, setAllowSites] = useState(false);

  if (isLoading || error) {
    return <div />;
  }

  return (
    <Create
      transform={(record: any) => {
        if (!allowDevices) {
          delete record.vpc_id;
          delete record.security_group_id;
        }
        if (!allowSites) {
          delete record.service_network_id;
        }
        return record;
      }}
    >
      <SimpleForm>
        <TextInput
          label="Description"
          name="description"
          source="description"
          fullWidth
        />
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

        {flags["devices"] && (
          <>
            <FormControlLabel
              label="Allow Device Registration"
              control={
                <Checkbox
                  checked={allowDevices}
                  onChange={(x) => {
                    setAllowDevices(x.target.checked);
                  }}
                />
              }
            />
            {allowDevices && (
              <Paper sx={{ width: "100%", marginTop: "1em" }}>
                <Card>
                  <CardContent>
                    <ReferenceInput
                      name="vpc_id"
                      source="vpc_id"
                      reference="vpcs"
                    >
                      <AutocompleteInput fullWidth />
                    </ReferenceInput>
                    <ReferenceInput
                      name="security_group_id"
                      source="security_group_id"
                      reference="security-groups"
                    >
                      <AutocompleteInput fullWidth />
                    </ReferenceInput>
                  </CardContent>
                </Card>
              </Paper>
            )}
          </>
        )}

        {flags["sites"] && (
          <>
            <FormControlLabel
              label="Allow Site Registration"
              control={
                <Checkbox
                  checked={allowSites}
                  onChange={(x) => {
                    setAllowSites(x.target.checked);
                  }}
                />
              }
            />
            {allowSites && (
              <Paper sx={{ width: "100%", marginTop: "1em" }}>
                <Card>
                  <CardContent>
                    <ReferenceInput
                      name="service_network_id"
                      source="service_network_id"
                      reference="service-networks"
                    >
                      <AutocompleteInput fullWidth />
                    </ReferenceInput>
                  </CardContent>
                </Card>
              </Paper>
            )}
          </>
        )}
      </SimpleForm>
    </Create>
  );
};

export const RegKeyEdit = () => {
  const flags = useFlags();
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
        {flags["security-groups"] && (
          <ReferenceInput
            name="security_group_id"
            source="security_group_id"
            reference="security-groups"
          >
            <AutocompleteInput fullWidth />
          </ReferenceInput>
        )}
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
        {/*
        <JsonInput
          label="Settings"
          name="settings"
          source="settings"
          fullWidth
          multiline={true}
        />
        */}
      </SimpleForm>
    </Edit>
  );
};
