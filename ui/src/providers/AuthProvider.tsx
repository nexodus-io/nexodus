import { AuthProvider, UserIdentity } from "react-admin";

// TODO: try and get this value dynamically from the token in auth header
// ideally we do not rely on ra for determining the token expiry.
// The keycloak token lifespan default for accessTokenLifespan is 300s
const TOKEN_EXPIRY = 300 * 1000;
// Updated refresh interval to be 10 seconds less than the token expiry
const REFRESH_INTERVAL_MS = TOKEN_EXPIRY - 10000;
const postHeaders = {
  "Content-Type": "application/json",
};
let refreshIntervalId: number;

const cleanup = () => {
  // Remove the ?code&state from the URL
  window.history.replaceState(
    {},
    window.document.title,
    window.location.origin,
  );
};

export const goOidcAgentAuthProvider = (api: string): AuthProvider => ({
  login: async (params = {}) => {
    // 1. Redirect to the issuer to ask authentication
    console.log("Redirecting to the issuer for authentication");
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
        console.error("Network error during login start:", err);
      }
    }

    // 2. We came back from the issuer with ?code infos in query params
    console.log("Handling return from issuer with code in query params");
    const request = new Request(`${api}/web/login/end`, {
      method: "POST",
      credentials: "include",
      headers: postHeaders,
      body: JSON.stringify({ request_url: window.location.href }),
    });
    try {
      const response = await fetch(request);
      const data = await response.json();
      console.log("Login end response data:", data);
      if (response && data) {
        // Call refreshToken once after the refresh interval expires (5 seconds less than the token expiry)
        setTimeout(() => refreshToken(api), REFRESH_INTERVAL_MS);
        // Then set up an interval to refresh the token 5 seconds less than the token expiry
        refreshIntervalId = setInterval(
          () => refreshToken(api),
          REFRESH_INTERVAL_MS,
        );
        return data.handled && data.logged_in
          ? Promise.resolve()
          : Promise.reject();
      }
    } catch (err: any) {
      console.error("Error during login end:", err);
      throw new Error(err.statusText);
    }
  },

  logout: async () => {
    console.log("Attempting logout");
    const request = new Request(`${api}/web/logout`, {
      method: "post",
      credentials: "include",
      headers: postHeaders,
    });
    try {
      const response = await fetch(request);
      console.log("Logout response data:", response);
      // Stop the token refresh interval
      if (refreshIntervalId) {
        console.log("Clearing refresh interval");
        clearInterval(refreshIntervalId);
      }
      cleanup();
      if (response.status === 401) {
        return Promise.resolve();
      }
      const data = await response.json();
      if (response && data) {
        window.location.replace(data.logout_url);
      }
    } catch (err: any) {
      console.error("Error during logout:", err);
      throw new Error(err.statusText);
    }
  },

  checkError: async (error: any) => {
    console.log("Checking error:", error);
    const status = error.status;
    if (status === 401) {
      return Promise.reject();
    }
    return Promise.resolve();
  },

  checkAuth: async (params) => {
    try {
      console.log("Starting checkAuth with params:", params);

      // Making the API request to check authentication
      const request = new Request(`${api}/web/check_auth`, {
        method: "GET",
        credentials: "include",
      });

      const response = await fetch(request);

      if (response.status === 401) {
        console.debug("User is not authenticated, trying to refresh token");

        // Making a call to Refresh the token
        console.debug("Attempting to refresh");

        const refreshRequest = new Request(`${api}/web/refresh`, {
          method: "GET",
          credentials: "include",
        });

        const refreshResponse = await fetch(refreshRequest);

        if (refreshResponse.status === 204) {
          console.debug("Token refreshed successfully");
          return Promise.resolve();
        } else if (refreshResponse.ok) {
          console.debug("Token refreshed successfully");
          // Since we are handling tokens on the server-side, no need to update client-side storage.
          return Promise.resolve();
        } else {
          console.debug("Failed to refresh the token");
          return Promise.reject(new Error("Failed to refresh the token"));
        }
      } else if (response.ok) {
        const data = await response.json();
        console.debug("checkAuth response:", response, "data:", data);
        return Promise.resolve();
      } else {
        console.debug("Unexpected status code:", response.status);
        return Promise.reject(
          new Error(`Unexpected status code: ${response.status}`),
        );
      }
    } catch (error) {
      console.error("An error occurred during the checkAuth:", error);
      return Promise.reject(error);
    }
  },

  getPermissions: async () => {
    console.log("getPermissions");
    // TODO: Add a callback so people can decode the claims
    return Promise.resolve();
  },

  getIdentity: async (): Promise<UserIdentity> => {
    console.log("Fetching identity");
    const request = new Request(`${api}/web/user_info`, {
      method: "GET",
      credentials: "include",
    });
    var id;
    try {
      const response = await fetch(request);
      const data = await response.json();
      console.log("Identity response data:", data);
      if (response && data) {
        id = {
          id: data.subject,
          fullName: data.preferred_username,
          avatar: data.picture,
        } as UserIdentity;
      }
    } catch (err: any) {
      console.error("Error  getting identity:", err);
      throw new Error(err.statusText);
    }
    if (id) {
      return Promise.resolve(id);
    }
    return Promise.reject();
  },
});

const refreshToken = async (api: string) => {
  console.log("Attempting to refresh the token.");
  const refreshRequest = new Request(`${api}/web/refresh`, {
    method: "GET",
    credentials: "include",
  });
  try {
    const refreshResponse = await fetch(refreshRequest);
    console.log("Token refresh response:", refreshResponse);

    if (refreshResponse.status === 204 || refreshResponse.ok) {
      console.debug("Token refreshed successfully.");
    } else {
      console.debug("Failed to refresh the token, clearing refresh interval");
      clearInterval(refreshIntervalId); // Clear the interval if the refresh fails
    }
  } catch (error) {
    console.error("An error occurred during token refresh:", error);
  }
};
