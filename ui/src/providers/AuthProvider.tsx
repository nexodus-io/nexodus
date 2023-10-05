import { AuthProvider, UserIdentity } from "react-admin";
import { RefreshManager } from "./RefreshManager";

const cleanup = () => {
  RefreshManager.stopRefreshing();
  console.log("Cleanup Called");
  // Remove the ?code&state from the URL
  window.history.replaceState(
    {},
    window.document.title,
    window.location.origin,
  );
  console.log("Window location after cleanup:", window.location.href);
};

export const goOidcAgentAuthProvider = (api: string): AuthProvider => ({
  login: async (params = {}) => {
    // 1. Redirect to the issuer to ask authentication
    if (!params.code || !params.state) {
      const request = new Request(`${api}/web/login/start`, {
        method: "POST",
        credentials: "include",
      });
      try {
        const response = await fetch(request);
        const data = await response.json();
        if (response && data) {
          window.location.replace(data.authorization_request_url);
        }
      } catch (err: any) {
        throw new Error("Network error");
      }
    }

    // 2. We came back from the issuer with ?code infos in query params
    const request = new Request(`${api}/web/login/end`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ request_url: window.location.href }),
    });
    try {
      const response = await fetch(request);
      const data = await response.json();
      if (response.ok && data) {
        if (data.access_token !== null) {
          localStorage.setItem("AccessToken", data.access_token);
          console.debug(`Stored Access Token: ${data.access_token}`);
        }
        if (data.refresh_token !== null) {
          localStorage.setItem("RefreshToken", data.refresh_token);
          console.debug(`Stored Refresh Token: ${data.refresh_token}`);
        }
        return data.handled && data.logged_in
          ? Promise.resolve()
          : Promise.reject();
      } else {
        console.log("Login failed:", response.statusText);
        cleanup();
        throw new Error("Login failed");
      }
    } catch (err: any) {
      console.log("Login Error:", err);
      cleanup();
      throw new Error(err.statusText || "Unknown error");
    }
  },

  logout: async () => {
    const request = new Request(`${api}/web/logout`, {
      method: "post",
      credentials: "include",
    });
    try {
      const response = await fetch(request);
      if (response.status === 401) {
        cleanup(); // Run cleanup if status is 401
        return Promise.resolve();
      } else if (response.status !== 200) {
        // If the status is neither 401 nor 200, something went wrong.
        cleanup();
        throw new Error(`Logout failed with status ${response.status}`);
      }
      const data = await response.json();
      if (response && data) {
        window.location.replace(data.logout_url);
      } else {
        // If the response object or data is somehow null or undefined, call cleanup().
        cleanup();
        throw new Error(
          "Logout failed, response or data is null or undefined.",
        );
      }
    } catch (err: any) {
      // If an exception is thrown while trying to logout, call the cleanup function.
      cleanup();
      throw new Error(err.statusText);
    }
  },

  checkError: async (error: any) => {
    const status = error.status;
    if (status === 401) {
      return Promise.reject();
    }
    return Promise.resolve();
  },

  checkAuth: async () => {
    console.log("Check Auth Called");
    const token = localStorage.getItem("AccessToken");

    if (!token) {
      console.debug("Token not found, calling /web/login/end");
      const request = new Request(`${api}/web/login/end`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ request_url: window.location.href }),
      });

      try {
        const response = await fetch(request);
        const data = await response.json();
        return data.logged_in ? Promise.resolve() : Promise.reject();
      } catch (err: any) {
        console.log("Error during login:", err);
        return Promise.reject();
      }
    }

    if (token) {
      const refreshToken = localStorage.getItem("RefreshToken");
      if (!refreshToken) {
        console.debug("No refresh token found. Cannot start refresh.");
        return Promise.reject();
      }

      if (!RefreshManager.hasStartedRefreshInterval) {
        console.debug("Starting the refresh interval.");

        const intervalId = window.setInterval(() => {
          RefreshManager.startRefreshing(api);
        }, RefreshManager.REFRESH_INTERVAL_MS);

        // Update RefreshManager's state
        RefreshManager.setRefreshIntervalId(intervalId);
        RefreshManager.setHasStartedRefreshInterval(true);
      }

      try {
        const refreshToken = localStorage.getItem("RefreshToken");
        if (!refreshToken) {
          console.log("No refresh token found. Cannot refresh the token.");
          return Promise.reject();
        }
        await RefreshManager.refreshToken(api);
        return Promise.resolve();
      } catch (err) {
        console.log("Error refreshing the token:", err);
        return Promise.reject();
      }
    }
  },

  getPermissions: async () => {
    console.log("Get Permissions Called");
    // TODO: Add a callback so people can decode the claims
    return Promise.resolve();
  },

  getIdentity: async (): Promise<UserIdentity> => {
    console.log("Get Identity Called");
    const request = new Request(`${api}/web/user_info`, {
      credentials: "include",
    });
    let id;
    try {
      const response = await fetch(request);
      const data = await response.json();
      if (response && data) {
        id = {
          id: data.subject,
          fullName: data.preferred_username,
          avatar: data.picture,
        } as UserIdentity;
      }
    } catch (err: any) {
      throw new Error(err.statusText);
    }
    if (id) {
      return Promise.resolve(id);
    }
    return Promise.reject();
  },
});
