import { goOidcAgentAuthProvider } from "./providers/AuthProvider";
import simpleRestProvider from "ra-data-simple-rest";
import { fetchUtils } from "react-admin";
import {
  DeleteParams,
  GetListParams,
  GetListResult,
  RaRecord,
} from "ra-core/dist/cjs/types";

const fetchJson = (url: string, options: any = {}) => {
  // Includes the encrypted session cookie in requests to the API
  options.credentials = "include";
  // some of the PUT api calls should be converted to PATCH
  if (options.method === "PUT") {
    if (
      url.startsWith(`${backend}/api/reg-keys/`) ||
      url.startsWith(`${backend}/api/devices/`) ||
      url.startsWith(`${backend}/api/security-groups/`) ||
      url.startsWith(`${backend}/api/vpcs/`)
    ) {
      options.method = "PATCH";
    }
  }
  return fetchUtils.fetchJson(url, options);
};
const backend = `${window.location.protocol}//api.${window.location.host}`;
export const authProvider = goOidcAgentAuthProvider(backend);
const baseDataProvider = simpleRestProvider(
  `${backend}/api`,
  fetchJson,
  "X-Total-Count",
);

function rewriteNestedResource(
  resource: string,
  params: { id?: any; meta?: any },
) {
  const resourceParts = resource.split("/");
  if (resourceParts.length > 1) {
    // to deal with nested resources
    let ids: any[] | undefined = undefined;
    if (params.id !== undefined) {
      ids = [...params.id];
    } else if (params.meta?.ids !== undefined) {
      ids = [...params.meta.ids];
    }
    if (ids === undefined) {
      throw new Error("meta.ids or id is required to access nested resources");
    }

    if (ids.length < resourceParts.length - 1) {
      throw new Error(
        `meta.ids should contain at least ${resourceParts.length - 1} elements`,
      );
    }

    const parts = [resourceParts[0]];
    for (let i = 0; i < resourceParts.length - 1; i++) {
      parts.push(ids[i]);
      parts.push(resourceParts[i + 1]);
    }
    resource = parts.join("/");

    if (params.id !== undefined) {
      params.id = params.id.pop();
    }
  }
  return resource;
}

async function deleteResource(resource: string, params: DeleteParams) {
  resource = rewriteNestedResource(resource, params);
  let result = await baseDataProvider.delete(resource, params);
  if (params.meta?.importer) {
    result.data = params.meta.importer(result.data);
  }
  return result;
}

export const dataProvider = {
  ...baseDataProvider,

  getFlag: (name: string) => {
    return fetchJson(`${backend}/api/fflags/${name}`).then(
      (response) => response,
    );
  },
  getFlags: async () => {
    return (await fetchJson(`${backend}/api/fflags`)).json as {
      [index: string]: boolean;
    };
  },

  delete: deleteResource,

  deleteMany: function (resource: string, params: any) {
    return Promise.all(
      params.ids.map(function (id: any) {
        return deleteResource(resource, { ...params, id: id });
      }),
    ).then(function (responses) {
      return {
        data: responses.map(function (a) {
          return a.data.id;
        }),
      };
    });
  },

  getList: async <RecordType extends RaRecord = any>(
    resource: string,
    params: GetListParams,
  ): Promise<GetListResult<RecordType>> => {
    resource = rewriteNestedResource(resource, params);

    let result = await baseDataProvider.getList(resource, params);

    if (params.meta?.importer) {
      result.data = result.data.map((record: any) => {
        return params.meta.importer(record);
      });
    }

    return result;
  },
};
