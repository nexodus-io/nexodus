import { useState, useRef, useEffect } from 'react';
import {
  Admin,
  AuthProvider,
  DataProvider,
  Resource,
} from 'react-admin';
import simpleRestProvider from 'ra-data-simple-rest';

// Auth
import Keycloak, {
  KeycloakConfig,
  KeycloakTokenParsed,
  KeycloakInitOptions,
} from 'keycloak-js';
import { keycloakAuthProvider, httpClient } from 'ra-keycloak';

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
import Layout from "./layout/Layout";

const config : KeycloakConfig = {
  url: import.meta.env.VITE_KEYCLOAK_URL,
  realm: import.meta.env.VITE_KEYCLOAK_REALM,
  clientId: import.meta.env.VITE_KEYCLOAK_CLIENT_ID,
};

const initOptions: KeycloakInitOptions = { onLoad: 'login-required' };

const getPermissions = (decoded: KeycloakTokenParsed) => {
    const roles = decoded?.realm_access?.roles;
    if (!roles) {
        return false;
    }
    if (roles.includes('admin')) return 'admin';
    if (roles.includes('user')) return 'user';
    return false;
};

const App = () => {
    const [keycloak, setKeycloak] = useState<Keycloak | undefined>(undefined);
    const authProvider = useRef<AuthProvider | undefined>(undefined);
    const dataProvider = useRef<DataProvider | undefined>(undefined);

    useEffect(() => {
        const initKeyCloakClient = async () => {
            const keycloakClient = new Keycloak(config);
            await keycloakClient.init(initOptions);
            authProvider.current = keycloakAuthProvider(keycloakClient, {
                onPermissions: getPermissions,
            });
            dataProvider.current = simpleRestProvider(import.meta.env.VITE_CONTROLLER_URL, httpClient(keycloakClient), 'X-Total-Count');
            setKeycloak(keycloakClient);
        };
        if (!keycloak) {
            initKeyCloakClient();
        }
    }, [keycloak]);

    // hide the admin until the dataProvider and authProvider are ready
    if (!keycloak) return <p>Loading...</p>;

  return (
    <Admin
      dashboard={Dashboard}
      authProvider={authProvider.current}
      dataProvider={dataProvider.current}
      title="Controller"
      layout={Layout}
    >
      <Resource name="users" list={UserList} show={UserShow} icon={UserIcon} />
      <Resource name="devices" list={DeviceList} show={DeviceShow} icon={DeviceIcon} />
      <Resource name="zones" list={ZoneList} show={ZoneShow} create={ZoneCreate} icon={ZoneIcon} />
      <Resource name="peers" list={PeerList} show={PeerShow} icon={UserIcon} />
    </Admin>
  );
};
export default App;
