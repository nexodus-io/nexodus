import {
  Admin,
  Resource,
  fetchUtils
} from 'react-admin';
import simpleRestProvider from 'ra-data-simple-rest';
import { goOidcAgentAuthProvider } from './providers/AuthProvider';

// icons
import DeviceIcon from '@mui/icons-material/Devices';
import ZoneIcon from '@mui/icons-material/VpnLock';
import UserIcon from '@mui/icons-material/People';

// pages
import { ZoneCreate, ZoneShow, ZoneList } from "./pages/Zones";
import { PeerShow, PeerList } from "./pages/Peers";
import { UserShow, UserList } from "./pages/Users";
import { DeviceList, DeviceShow } from "./pages/Devices";
import Dashboard from './pages/Dashboard';
import LoginPage from './pages/Login';
import Layout from "./layout/Layout";

const fetchJson = (url: URL, options: any = {}) => {
  // Includes the encrypted session cookie in requests to the API
  options.credentials = "include";
  return fetchUtils.fetchJson(url, options);
};

const backend = `${window.location.protocol}//api.${window.location.host}`;
const authProvider = goOidcAgentAuthProvider(backend);
const dataProvider = simpleRestProvider(
  `${backend}/api`,
  fetchJson,
  'X-Total-Count',
);

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
      <Resource name="users" list={UserList} show={UserShow} icon={UserIcon} recordRepresentation={(record) => `${record.username}`} />
      <Resource name="devices" list={DeviceList} show={DeviceShow} icon={DeviceIcon} recordRepresentation={(record) => `${record.hostname}`} />
      <Resource name="zones" list={ZoneList} show={ZoneShow} create={ZoneCreate} icon={ZoneIcon} recordRepresentation={(record) => `${record.name}`} />
      <Resource name="peers" list={PeerList} show={PeerShow} icon={UserIcon} />
    </Admin>
  );
};
export default App;
