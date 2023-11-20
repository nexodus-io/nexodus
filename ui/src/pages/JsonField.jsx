import React, { useCallback } from "react";
import {
  Identifier,
  RaRecord,
  TextField,
  TextInput,
  useRecordContext,
  UseRecordContextParams,
  useRegisterMutationMiddleware,
} from "react-admin";

export const JsonField = (props) => {
  const record = useRecordContext(props);
  let newRecord = { ...record };
  newRecord[props.source] = JSON.stringify(record[props.source], null, 2);
  return record ? <TextField {...props} record={newRecord} /> : null;
};
