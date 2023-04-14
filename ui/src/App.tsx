import { Admin, Resource, fetchUtils } from "react-admin";
import simpleRestProvider from "ra-data-simple-rest";
import { goOidcAgentAuthProvider } from "./providers/AuthProvider";

// icons
import DeviceIcon from "@mui/icons-material/Devices";
import OrganizationIcon from "@mui/icons-material/VpnLock";
import UserIcon from "@mui/icons-material/People";

// pages
import { UserShow, UserList } from "./pages/Users";
import { DeviceList, DeviceShow } from "./pages/Devices";
import { OrganizationList, OrganizationShow } from "./pages/Organizations";
import Dashboard from "./pages/Dashboard";
import LoginPage from "./pages/Login";
import Layout from "./layout/Layout";

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
  "X-Total-Count"
);
const dataProvider = {
  ...baseDataProvider,
  getFlag: (name: string) => {
    return fetchJson(new URL(`${backend}/api/fflags/${name}`)).then(
      (response) => response
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
      requireAuth
    >
      <Resource
        name="users"
        list={UserList}
        show={UserShow}
        icon={UserIcon}
        recordRepresentation={(record) => `${record.username}`}
      />
      <Resource
        name="organizations"
        list={OrganizationList}
        show={OrganizationShow}
        icon={OrganizationIcon}
        recordRepresentation={(record) => `${record.name}`}
      />
      <Resource
        name="organizations"
        list={OrganizationList}
        show={OrganizationShow}
        icon={OrganizationIcon}
        recordRepresentation={(record) => `${record.name}`}
      />
      <Resource
        name="devices"
        list={DeviceList}
        show={DeviceShow}
        icon={DeviceIcon}
        recordRepresentation={(record) => `${record.hostname}`}
      />
    </Admin>
  );
};
export default App;
