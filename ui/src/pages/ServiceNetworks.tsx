import { Fragment } from "react";
import {
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

const ServiceNetworkListBulkActions = () => (
  <Fragment>
    <BulkExportButton />
    <BulkDeleteButton />
  </Fragment>
);

export const ServiceNetworkList = () => {
  const flags = useFlags();
  return (
    <List>
      <Datagrid
        rowClick="show"
        bulkActionButtons={<ServiceNetworkListBulkActions />}
      >
        <TextField label="Description" source="description" />
        {flags["sites"] && (
          <div style={{ display: "flex", alignItems: "center" }}>
            <ReferenceManyCount
              label="Sites"
              reference="sites"
              target="service_network_id"
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

export const ServiceNetworkShow = () => {
  const flags = useFlags();
  return (
    <Show>
      <SimpleShowLayout>
        <TextField label="ID" source="id" />
        <TextField label="Description" source="description" />
        {flags["sites"] && (
          <div style={{ display: "block", marginBottom: "1rem" }}>
            <ReferenceManyField
              label="Registered Sites"
              reference="sites"
              target="service_network_id"
            >
              <Datagrid>
                <TextField label="Hostname" source="hostname" />
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

export const ServiceNetworkCreate = () => {
  const flags = useFlags();
  return (
    <Create>
      <SimpleForm>
        <TextInput label="Description" source="description" fullWidth />
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
