import * as React from "react";
import { Admin, Resource} from 'react-admin';
import { fetchUtils } from 'ra-core';
import simpleRestProvider from 'ra-data-simple-rest';
import { PeerList, PeerShow } from "./peers";
import { ZoneCreate, ZoneShow, ZoneList } from "./zones";
import DeviceIcon from '@mui/icons-material/Devices';
import ZoneIcon from '@mui/icons-material/VpnLock';
import PeerIcon from '@mui/icons-material/Link';
import { DeviceList, DeviceShow } from "./devices";

const dataProvider = simpleRestProvider('http://localhost:8080', fetchUtils.fetchJson, 'X-Total-Count');

const App = () => (
  <Admin dataProvider={dataProvider}>
    <Resource name="peers" list={PeerList} show={PeerShow} icon={PeerIcon}/>
    <Resource name="zones" list={ZoneList} show={ZoneShow} create={ZoneCreate} icon={ZoneIcon}/>
    <Resource name="devices" list={DeviceList} show={DeviceShow} icon={DeviceIcon}/>
  </Admin>
);
export default App;
