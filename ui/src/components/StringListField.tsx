import {
  Identifier,
  RaRecord,
  RecordContextProvider,
  useRecordContext,
  UseRecordContextParams,
} from "react-admin";

export const StringListField = (
  props: UseRecordContextParams<RaRecord<Identifier>>,
) => {
  const record = useRecordContext(props);
  return (
    <div>
      {record[props.source].map((item: any) => {
        const value = props.mappper ? props.mappper(item) : item;
        return (
          <RecordContextProvider key={item} value={{ value: value }}>
            {props.children}
          </RecordContextProvider>
        );
      })}
    </div>
  );
};
