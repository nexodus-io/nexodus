import React, { useCallback } from "react";
import { TextInput, useRegisterMutationMiddleware } from "react-admin";

export const JsonInput = (props) => {
  const middleware = useCallback(
    async (resource, params, options, next) => {
      const data = { ...params.data };
      let field = data[props.source];
      if (field && field.trim() !== "") {
        field = JSON.parse(field);
      } else {
        field = null;
      }
      data[props.source] = field;
      const newParams = { ...params, data: data };
      await next(resource, newParams, options);
    },
    [props.source],
  );
  useRegisterMutationMiddleware(middleware);
  return (
    <TextInput
      {...props}
      format={(x) => {
        if (x && typeof x === "object") {
          return JSON.stringify(x, null, 2);
        }
        return x;
      }}
    />
  );
};
