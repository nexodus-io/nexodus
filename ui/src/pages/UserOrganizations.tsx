import {
  BulkDeleteButton,
  BulkExportButton,
  ChipField,
  Datagrid,
  List,
  TextField,
} from "react-admin";
import { useParams } from "react-router-dom";
import React, { Fragment } from "react";
import { StringListField } from "../components/StringListField";
import { choiceMapper, roleChoices } from "./Invitations";

function importer(record: any) {
  record.id = [record.organization_id, record.user_id];
  return record;
}

export const UserOrganizationList = () => {
  const { id } = useParams();

  return (
    <List
      resource="organizations/users"
      queryOptions={{ meta: { ids: [id], importer } }}
      sort={{ field: "user_id", order: "ASC" }}
    >
      <Datagrid
        bulkActionButtons={
          <Fragment>
            <BulkExportButton />
            <BulkDeleteButton />
          </Fragment>
        }
      >
        <TextField label="Full Name" source="user.full_name" />
        <TextField label="User Name" source="user.username" />
        <StringListField
          source="roles"
          label="Roles"
          mappper={choiceMapper(roleChoices)}
        >
          <ChipField source="value" size="small" />
        </StringListField>
      </Datagrid>
    </List>
  );
};
