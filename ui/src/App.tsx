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
import PeerIcon from '@mui/icons-material/Link';
// pages
import { PeerList, PeerShow } from "./pages/peers";
import { ZoneCreate, ZoneShow, ZoneList } from "./pages/zones";
import { DeviceList, DeviceShow } from "./pages/devices";
import { MyLayout } from "./components/layout";

const config : KeycloakConfig = {
  url: 'http://localhost:8888',
  realm: 'controltower',
  clientId: 'front-controltower',
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
            dataProvider.current = simpleRestProvider('http://localhost:8080', httpClient(keycloakClient), 'X-Total-Count');
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
      authProvider={authProvider.current}
      dataProvider={dataProvider.current}
      title="Controltower"
      layout={MyLayout}
    >
      <Resource name="peers" list={PeerList} show={PeerShow} icon={PeerIcon} />
      <Resource name="zones" list={ZoneList} show={ZoneShow} create={ZoneCreate} icon={ZoneIcon} />
      <Resource name="devices" list={DeviceList} show={DeviceShow} icon={DeviceIcon} />
    </Admin>
  );
};
export default App;
