import * as React from "react";
import { Admin, Resource} from 'react-admin';
import { fetchUtils } from 'ra-core';
import simpleRestProvider from 'ra-data-simple-rest';
import { PeerList, PeerShow } from "./peers";
import { ZoneCreate, ZoneShow, ZoneList } from "./zones";
import { PubKeyList, PubKeyShow } from "./pubkeys";
import PubKeyIcon from '@mui/icons-material/Key';
import ZoneIcon from '@mui/icons-material/VpnLock';
import PeerIcon from '@mui/icons-material/Spoke';

const dataProvider = simpleRestProvider('http://localhost:8080', fetchUtils.fetchJson, 'X-Total-Count');

const App = () => (
  <Admin dataProvider={dataProvider}>
    <Resource name="peers" list={PeerList} show={PeerShow} icon={PeerIcon}/>
    <Resource name="zones" list={ZoneList} show={ZoneShow} create={ZoneCreate} icon={ZoneIcon}/>
    <Resource name="pubkeys" list={PubKeyList} show={PubKeyShow} icon={PubKeyIcon}/>
  </Admin>
);
export default App;
