import { AuthProvider, UserIdentity } from "react-admin";

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
        console.log("Login start response data:", data);
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
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ request_url: window.location.href }),
    });
    try {
      const response = await fetch(request);
      const data = await response.json();
      console.log("Login end response data:", data);
      if (response && data) {
        cleanup();
        return data.handled && data.logged_in
          ? Promise.resolve()
          : Promise.reject();
      }
    } catch (err: any) {
      cleanup();
      console.error("Error during login end:", err);
      throw new Error(err.statusText);
    }
  },
  logout: async () => {
    console.log("Attempting logout");
    const request = new Request(`${api}/web/logout`, {
      method: "post",
      credentials: "include",
    });
    try {
      const response = await fetch(request);
      console.log("Logout response data:", response);

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

  checkAuth: async () => {
    console.log("Checking authentication");
    const request = new Request(`${api}/web/check_auth`, {
      method: "GET",
      credentials: "include",
    });
    const response = await fetch(request);
    const data = await response.json();

    if (response.status === 401) {
      // User is not authenticated
      console.log(data.message);
      return Promise.reject();
    } else if (response.status === 200) {
      // User is authenticated
      console.log(data.message);
      return Promise.resolve();
    } else {
      console.error("Unexpected response during checkAuth:", response);
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
