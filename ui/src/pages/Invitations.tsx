import React, { Fragment, FunctionComponent, useCallback } from "react";
import {
  BulkDeleteButton,
  BulkExportButton,
  Button,
  Create,
  Datagrid,
  List,
  ReferenceInput,
  required,
  Show,
  SimpleForm,
  SimpleShowLayout,
  TextField,
  TextInput,
  useGetIdentity,
  useNotify,
  useRefresh,
  useRecordContext,
  NotificationType,
  UseRecordContextParams,
  RaRecord,
  Identifier,
  DateTimeInput,
  AutocompleteInput,
  CheckboxGroupInput,
  ArrayField,
  SingleFieldList,
  ChipField,
  WrapperField,
} from "react-admin";

import { backend, fetchJson as apiFetchJson } from "../common/Api";
import { StringListField } from "../components/StringListField";

export const roleChoices = [
  { id: "owner", name: "Owner" },
  { id: "member", name: "Member" },
];

export function choiceMapper(choices: { id: string; name: string }[]) {
  return function (x: any) {
    const choice = choices.find((c) => c.id === x);
    return choice ? choice.name : x;
  };
}

const InvitationListBulkActions = () => (
  <Fragment>
    <BulkExportButton />
    <BulkDeleteButton />
  </Fragment>
);

const AcceptInvitationButton: FunctionComponent = () => {
  const record = useRecordContext<{ id?: number }>();
  const notify = useNotify();
  const refresh = useRefresh();

  const handleAccept = useCallback(async () => {
    if (!record || !record.id) {
      console.error("Record or record ID is undefined");
      notify("No record selected for accepting the invitation", {
        type: "warning" as NotificationType,
      });
      return;
    }

    console.log("Attempting to accept invitation for record ID:", record.id);

    try {
      const response = await apiFetchJson(
        `${backend}/api/invitations/${record.id}/accept`,
        { method: "POST" },
      );
      console.log("Invitation accept response:", response);
      notify("Invitation accepted", { type: "info" as NotificationType });
      refresh();
    } catch (error) {
      if (error instanceof Error) {
        console.error("Error accepting invitation:", error.message);
        notify("Error accepting invitation: " + error.message, {
          type: "warning" as NotificationType,
        });
      } else {
        console.error("Error accepting invitation:", error);
        notify("Error accepting invitation", {
          type: "warning" as NotificationType,
        });
      }
    }
  }, [record, notify, refresh]);

  return <Button label="Accept" onClick={handleAccept} />;
};

export const AcceptInvitationField = (
  props: UseRecordContextParams<RaRecord<Identifier>> | undefined,
) => {
  const record = useRecordContext(props);
  const { identity } = useGetIdentity();
  if (!identity) {
    return null;
  }
  // only show the accept button for invitations that are for the current user
  return record && identity && identity.id == record.user_id ? (
    <AcceptInvitationButton />
  ) : null;
};

export const InvitationList = () => (
  <List>
    <Datagrid rowClick="show" bulkActionButtons={<InvitationListBulkActions />}>
      <TextField label="From" source="from.full_name" />
      <TextField label="To" source="email" />
      <TextField label="Organization" source="organization.name" />
      <TextField label="Expires At" source="expires_at" />
      <StringListField
        source="roles"
        label="Roles"
        mappper={choiceMapper(roleChoices)}
      >
        <ChipField source="value" size="small" />
      </StringListField>
      <AcceptInvitationField />
    </Datagrid>
  </List>
);

export const InvitationShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField label="ID" source="id" />
      <TextField label="From" source="from.full_name" />
      <TextField label="To" source="email" />
      <TextField label="Organization" source="organization.name" />
      <TextField label="Expires At" source="expires_at" />
      <StringListField
        source="roles"
        label="Roles"
        mappper={choiceMapper(roleChoices)}
      >
        <ChipField source="value" size="small" />
      </StringListField>
      <AcceptInvitationField />
    </SimpleShowLayout>
  </Show>
);

export const InvitationCreate = () => {
  const { identity, isLoading, error } = useGetIdentity();
  if (isLoading || error || !identity) {
    return <div />;
  }
  return (
    <Create>
      <SimpleForm>
        <TextInput
          label="Email Address"
          name="email"
          source="email"
          validate={[required()]}
          fullWidth
        />
        <ReferenceInput
          label="Organization"
          name="organization_id"
          source="organization_id"
          reference="organizations"
          filter={{ roles: ["owner"] }}
        >
          <AutocompleteInput fullWidth />
        </ReferenceInput>
        <DateTimeInput
          label="Expires At"
          name="expires_at"
          source="expires_at"
          fullWidth
        />
        <CheckboxGroupInput
          defaultValue={["member"]}
          label="Roles"
          source="roles"
          row={false}
          choices={roleChoices}
        />
      </SimpleForm>
    </Create>
  );
};
