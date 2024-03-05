import { Admin, CustomRoutes, Resource } from "react-admin";
import { Route } from "react-router";

// icons
import DeviceIcon from "@mui/icons-material/Devices";
import SiteIcon from "@mui/icons-material/BorderOuter";
import OrganizationIcon from "@mui/icons-material/People";
import UserIcon from "@mui/icons-material/Person";
import InvitationIcon from "@mui/icons-material/Rsvp";
import RegKeyIcon from "@mui/icons-material/Key";
import VPCIcon from "@mui/icons-material/Cloud";
import ServiceNetworkIcon from "@mui/icons-material/Cloud";

// pages
import { UserList, UserShow } from "./pages/Users";
import { DeviceEdit, DeviceList, DeviceShow } from "./pages/Devices";
import {
  OrganizationCreate,
  OrganizationList,
  OrganizationShow,
} from "./pages/Organizations";
import { VPCCreate, VPCList, VPCShow } from "./pages/VPCs";
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
import { authProvider, dataProvider } from "./DataProvider";
import { createTheme } from "@mui/material";
import { FlagsProvider } from "./common/FlagsContext";
import {
  ServiceNetworkCreate,
  ServiceNetworkList,
  ServiceNetworkShow,
} from "./pages/ServiceNetworks";

const darkTheme = createTheme({
  palette: { mode: "dark" },
});

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
      darkTheme={darkTheme}
      requireAuth
      disableTelemetry
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
        name="service-networks"
        list={ServiceNetworkList}
        show={ServiceNetworkShow}
        icon={ServiceNetworkIcon}
        create={ServiceNetworkCreate}
        recordRepresentation={(record) => `${record.description}`}
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
