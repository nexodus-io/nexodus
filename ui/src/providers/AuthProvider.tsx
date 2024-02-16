import { AuthProvider, UserIdentity } from "react-admin";
import { RefreshManager } from "./RefreshManager";
import { red } from "@mui/material/colors";

export const goOidcAgentAuthProvider = (api: string): AuthProvider => {
  const getMe = async (): Promise<UserIdentity> => {
    const request = new Request(`${api}/api/users/me`, {
      credentials: "include",
    });
    let id;
    try {
      const response = await fetch(request);
      if (!response || response.status !== 200) {
        return Promise.reject();
      }
      const data = await response.json();
      if (!data) {
        return Promise.reject();
      }

      return {
        id: data.id,
        fullName: data.full_name,
        avatar: data.picture,
        email: data.email,
      };
    } catch (err: any) {
      throw new Error(err.statusText);
    }
  };

  return {
    login: async (params = {}) => {
      console.log("Login Called!!");

      let redirect = window.location.href;
      if (redirect.endsWith("#/login")) {
        // replace #/login with empty string
        redirect = redirect.replace("#/login", "");
      }
      console.log("login", params);
      console.log("redirect", redirect);

      // Send the user to the authentication server, and have them come back to the redirect URL
      window.location.replace(`${api}/web/login/start?redirect=${redirect}`);
    },

    logout: async () => {
      console.log("Logout Called");
      try {
        await getMe();
      } catch (err) {
        // If we are not logged in, then we don't need to log out
        return;
      }

      RefreshManager.stopRefreshing();
      let redirect = window.location.href;
      // does the redirect contain a hash? If so, remove it.
      redirect = redirect.split("#")[0];
      window.location.replace(`${api}/web/logout?redirect=${redirect}`);
    },

    checkError: async (error: any) => {
      const status = error.status;
      if (status === 401) {
        return Promise.reject();
      }
      return Promise.resolve();
    },

    checkAuth: async () => {
      console.log("Check Auth Called!");
      await getMe();
      await RefreshManager.startRefreshing(api);
    },

    getIdentity: async (): Promise<UserIdentity> => {
      console.log("Get Identity Called");
      return await getMe();
    },

    getPermissions: async () => {
      console.log("Get Permissions Called");
      // TODO: Add a callback so people can decode the claims
      return Promise.resolve();
    },
  };
};
