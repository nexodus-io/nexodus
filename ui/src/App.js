import * as React from "react";
import { Admin, Resource } from 'react-admin';
import { fetchUtils } from 'ra-core';
import simpleRestProvider from 'ra-data-simple-rest';
import { PeerList, PeerShow } from "./peers";
import { ZoneCreate, ZoneShow, ZoneList } from "./zones";

const dataProvider = simpleRestProvider('http://localhost:8080', fetchUtils.fetchJson, 'X-Total-Count');

const App = () => (
  <Admin dataProvider={dataProvider}>
    <Resource name="peers" list={PeerList} show={PeerShow} />
    <Resource name="zones" list={ZoneList} show={ZoneShow} create={ZoneCreate} />
  </Admin>
);
export default App;
