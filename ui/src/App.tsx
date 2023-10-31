import { Admin, CustomRoutes, Resource, fetchUtils } from "react-admin";
import { Route } from "react-router";

import simpleRestProvider from "ra-data-simple-rest";
import { goOidcAgentAuthProvider } from "./providers/AuthProvider";

// icons
import DeviceIcon from "@mui/icons-material/Devices";
import OrganizationIcon from "@mui/icons-material/VpnLock";
import UserIcon from "@mui/icons-material/People";
import InvitationIcon from "@mui/icons-material/Rsvp";
import RegistrationTokenIcon from "@mui/icons-material/Rsvp";

// pages
import { UserShow, UserList } from "./pages/Users";
import { DeviceList, DeviceShow } from "./pages/Devices";
import {
  OrganizationList,
  OrganizationShow,
  OrganizationCreate,
} from "./pages/Organizations";
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
  RegistrationTokenCreate,
  RegistrationTokenList,
  RegistrationTokenShow,
} from "./pages/RegistrationTokens";

const fetchJson = (url: URL, options: any = {}) => {
  // Includes the encrypted session cookie in requests to the API
  options.credentials = "include";
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
    return fetchJson(new URL(`${backend}/api/fflags/${name}`)).then(
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
        <Route path="/security-groups" element={<SecurityGroups />} />
      </CustomRoutes>
      <Resource
        name="organizations"
        list={OrganizationList}
        show={OrganizationShow}
        icon={OrganizationIcon}
        create={OrganizationCreate}
        recordRepresentation={(record) => `${record.name}`}
      />
      <Resource
        name="devices"
        list={DeviceList}
        show={DeviceShow}
        icon={DeviceIcon}
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
        name="registration-tokens"
        list={RegistrationTokenList}
        show={RegistrationTokenShow}
        icon={RegistrationTokenIcon}
        create={RegistrationTokenCreate}
        recordRepresentation={(record) => `${record.id}`}
      />
    </Admin>
  );
};
export default App;
