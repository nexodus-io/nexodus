import { Admin, Resource, ListGuesser, fetchUtils } from "react-admin";
import jsonServerProvider from "ra-data-json-server";
import { goOidcAgentAuthProvider } from "./providers/AuthProvider";
import LoginPage from "./pages/LoginPage";

const fetchJson = (url: URL, options: any = {}) => {
  // Includes the encrypted session cookie in requests to the API
  options.credentials = "include";
  return fetchUtils.fetchJson(url, options);
};

const authProvider = goOidcAgentAuthProvider("http://api.widgets.local:8080");
const dataProvider = jsonServerProvider(
  "http://api.widgets.local:8080/api",
  fetchJson
);

const App = () => (
  <Admin
    authProvider={authProvider}
    dataProvider={dataProvider}
    loginPage={LoginPage}
    requireAuth
  >
    <Resource name="widgets" list={ListGuesser} />
  </Admin>
);
export default App;
