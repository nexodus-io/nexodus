import { AuthProvider, UserIdentity } from "react-admin";

const cleanup = () => {
  // Remove the ?code&state from the URL
  window.history.replaceState(
    {},
    window.document.title,
    window.location.origin
  );
};

export const goOidcAgentAuthProvider = (api: string): AuthProvider => ({
  login: async (params = {}) => {
    // 1. Redirect to the issuer to ask authentication
    if (!params.code || !params.state) {
      const request = new Request(`${api}/login/start`, {
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
    const request = new Request(`${api}/login/end`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ request_url: window.location.href }),
    });
    try {
      const response = await fetch(request);
      const data = await response.json();
      if (response && data) {
        cleanup();
        return data.handled && data.logged_in
          ? Promise.resolve()
          : Promise.reject();
      }
    } catch (err: any) {
      cleanup();
      throw new Error(err.statusText);
    }
  },
  logout: async () => {
    const request = new Request(`${api}/logout`, {
      method: "post",
      credentials: "include",
    });
    try {
      const response = await fetch(request);
      if (response.status === 401) {
        return Promise.resolve();
      }
      const data = await response.json();
      if (response && data) {
        window.location.replace(data.logout_url);
      }
    } catch (err: any) {
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
    const request = new Request(`${api}/login/end`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ request_url: window.location.href }),
    });
    try {
      const response = await fetch(request);
      const data = await response.json();
      if (response && data) {
        return data.logged_in ? Promise.resolve() : Promise.reject();
      }
    } catch (err: any) {
      throw new Error(err.statusText);
    }
  },
  getPermissions: async () => {
    console.log("getPermissions");
    // TODO: Add a callback so people can decode the claims
    return Promise.resolve();
  },
  getIdentity: async (): Promise<UserIdentity> => {
    const request = new Request(`${api}/user_info`, {
      credentials: "include",
    });
    var id;
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
