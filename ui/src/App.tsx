import { Admin, CustomRoutes, Resource, fetchUtils } from "react-admin";
import { Route } from "react-router";

import simpleRestProvider from "ra-data-simple-rest";
import { goOidcAgentAuthProvider } from "./providers/AuthProvider";

// icons
import DeviceIcon from "@mui/icons-material/Devices";
import SiteIcon from "@mui/icons-material/BorderOuter";
import OrganizationIcon from "@mui/icons-material/People";
import UserIcon from "@mui/icons-material/Person";
import InvitationIcon from "@mui/icons-material/Rsvp";
import RegKeyIcon from "@mui/icons-material/Key";
import VPCIcon from "@mui/icons-material/Cloud";

// pages
import { UserShow, UserList } from "./pages/Users";
import { DeviceEdit, DeviceList, DeviceShow } from "./pages/Devices";
import {
  OrganizationList,
  OrganizationShow,
  OrganizationCreate,
} from "./pages/Organizations";
import { VPCList, VPCShow, VPCCreate } from "./pages/VPCs";
import Dashboard from "./pages/Dashboard";
import LoginPage from "./pages/Login";
import Layout from "./layout/Layout";
import {
  InvitationCreate,
  InvitationList,
  InvitationShow,
} from "./pages/Invitations";
import SecurityGroups from "./pages/SecurityGroups/SecurityGroups";

// components
import { CustomMenu } from "./layout/Menus";
import {
  RegKeyCreate,
  RegKeyEdit,
  RegKeyList,
  RegKeyShow,
} from "./pages/RegKeys";
import { SiteEdit, SiteList, SiteShow } from "./pages/Sites";

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
const authProvider = goOidcAgentAuthProvider(backend);
const baseDataProvider = simpleRestProvider(
  `${backend}/api`,
  fetchJson,
  "X-Total-Count",
);
const dataProvider = {
  ...baseDataProvider,
  getFlag: (name: string) => {
    return fetchJson(`${backend}/api/fflags/${name}`).then(
      (response) => response,
    );
  },
};

const App = () => {
  return (
    <Admin
      dashboard={Dashboard}
      authProvider={authProvider}
      dataProvider={dataProvider}
      title="Controller"
      layout={Layout}
      loginPage={LoginPage}
      menu={CustomMenu}
      requireAuth
    >
      <Resource
        name="users"
        list={UserList}
        show={UserShow}
        icon={UserIcon}
        recordRepresentation={(record) => `${record.username}`}
      />
      <CustomRoutes>
        <Route path="/_security-groups" element={<SecurityGroups />} />
      </CustomRoutes>
      {/* define security-groups so that it can be used as a reference from other resources. */}
      <Resource
        name="security-groups"
        recordRepresentation={(record) => `${record.description}`}
      />
      <Resource
        name="organizations"
        list={OrganizationList}
        show={OrganizationShow}
        icon={OrganizationIcon}
        create={OrganizationCreate}
        recordRepresentation={(record) => `${record.name}`}
      />
      <Resource
        name="vpcs"
        list={VPCList}
        show={VPCShow}
        icon={VPCIcon}
        create={VPCCreate}
        recordRepresentation={(record) => `${record.description}`}
      />
      <Resource
        name="devices"
        list={DeviceList}
        show={DeviceShow}
        icon={DeviceIcon}
        edit={DeviceEdit}
        recordRepresentation={(record) => `${record.hostname}`}
      />
      <Resource
        name="sites"
        list={SiteList}
        show={SiteShow}
        icon={SiteIcon}
        edit={SiteEdit}
        recordRepresentation={(record) => `${record.hostname}`}
      />
      <Resource
        name="invitations"
        list={InvitationList}
        show={InvitationShow}
        icon={InvitationIcon}
        create={InvitationCreate}
        recordRepresentation={(record) => `${record.hostname}`}
      />
      <Resource
        name="reg-keys"
        list={RegKeyList}
        show={RegKeyShow}
        edit={RegKeyEdit}
        icon={RegKeyIcon}
        create={RegKeyCreate}
        recordRepresentation={(record) => `${record.id}`}
      />
    </Admin>
  );
};
export default App;
