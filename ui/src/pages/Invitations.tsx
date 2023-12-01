import { Fragment, FunctionComponent, useCallback } from "react";
import {
  BulkDeleteButton,
  BulkExportButton,
  Button,
  Create,
  Datagrid,
  List,
  ReferenceField,
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
} from "react-admin";

import { backend, fetchJson as apiFetchJson } from "../common/Api";

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

const AcceptInvitationField: FunctionComponent = () => {
  return <AcceptInvitationButton />;
};

export const InvitationList = () => (
  <List>
    <Datagrid rowClick="show" bulkActionButtons={<InvitationListBulkActions />}>
      <TextField label="ID" source="id" />
      <TextField label="Email Address" source="email" />
      {/* Right now we can't look up other users, we don't have access */}
      {/*<ReferenceField*/}
      {/*  label="User"*/}
      {/*  source="user_id"*/}
      {/*  reference="users"*/}
      {/*  link="show"*/}
      {/*/>*/}
      <ReferenceField
        label="Organization"
        source="organization_id"
        reference="organizations"
        link="show"
      />
      <TextField label="Expires" source="expiry" />
      <AcceptInvitationField />
    </Datagrid>
  </List>
);

export const InvitationShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField label="ID" source="id" />
      <TextField label="User ID" source="user_id" />
      {/* Right now we can't look up other users, we don't have access */}
      {/*<ReferenceField*/}
      {/*  label="User"*/}
      {/*  source="user_id"*/}
      {/*  reference="users"*/}
      {/*  link="show"*/}
      {/*/>*/}
      <ReferenceField
        label="Organization"
        source="organization_id"
        reference="organizations"
        link="show"
      />
      <TextField label="Expires" source="expiry" />
    </SimpleShowLayout>
  </Show>
);

export const InvitationCreate = () => {
  const { identity, isLoading, error } = useGetIdentity();
  if (isLoading || error) {
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
          label="User Name"
          name="organization_id"
          source="organization_id"
          reference="organizations"
          filter={{ owner_id: identity.id }}
        />
      </SimpleForm>
    </Create>
  );
};
