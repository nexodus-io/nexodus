import { goOidcAgentAuthProvider } from "./providers/AuthProvider";
import simpleRestProvider from "ra-data-simple-rest";
import { fetchUtils } from "react-admin";

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
};
