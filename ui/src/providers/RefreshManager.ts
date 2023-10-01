import { JwtDecodeExp, RefreshTokenResponse } from "./AuthTypes";
import jwtDecode from "jwt-decode";

export class RefreshManager {
  static hasStartedRefreshInterval = false;
  static refreshIntervalId: number | undefined;
  static REFRESH_INTERVAL_MS = 90 * 1000;

  static setRefreshIntervalId(intervalId: number) {
    this.refreshIntervalId = intervalId;
  }

  static setHasStartedRefreshInterval(value: boolean) {
    this.hasStartedRefreshInterval = value;
  }

  // one-time refresh POST
  static async postRefreshTokens(
    api: string,
  ): Promise<RefreshTokenResponse | null> {
    const refreshToken = localStorage.getItem("RefreshToken");
    const request = new Request(`${api}/web/refresh`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });

    try {
      const response = await fetch(request);
      if (response.ok) {
        const data: RefreshTokenResponse = await response.json();
        // Uncomment to print refresh tokens in console
        // console.debug("Refresh tokens new tokens:", data);
        if (data.access_token) {
          // Decode and print the expiration time of the new access token
          const decoded: JwtDecodeExp = jwtDecode(data.access_token);
          console.debug(
            "New access token expires at:",
            new Date(decoded.exp * 1000).toLocaleString(),
          );
        }
        return data;
      } else {
        console.error(
          `Received ${response.status} from server during token fetch.`,
        );
        if (
          response.headers.get("Content-Type")?.includes("application/json")
        ) {
          const errorData = await response.json();
          console.error("Error data from server:", errorData);
        }
        return null;
      }
    } catch (error) {
      console.error("An error occurred during token fetch:", error);
      return null;
    }
  }

  // long-running refreshing on a timer
  static async startRefreshing(api: string): Promise<void> {
    try {
      console.debug("Start Refreshing Firing.");
      const data = await this.postRefreshTokens(api);
      if (data && (data.access_token || data.refresh_token)) {
        console.debug("Token interval refreshed successfully.");
      } else {
        console.debug("Failed to refresh the token, clearing refresh interval");
        clearInterval(this.refreshIntervalId as any);
        this.hasStartedRefreshInterval = false;
      }
      console.debug("Refresh poller completed.");
    } catch (e) {
      console.error("Error in startRefreshing:", e);
    }
  }

  static async refreshToken(api: string): Promise<void> {
    const data = await this.postRefreshTokens(api);
    if (data) {
      this.handleTokenResponse(data);
    } else {
      console.log("Failed to refresh the token.");
      return Promise.reject();
    }
  }

  // Store the tokens in local storage and log the expiration date
  static handleTokenResponse(data: RefreshTokenResponse) {
    if (data.refresh_token) {
      const decodedToken = jwtDecode<JwtDecodeExp>(data.refresh_token!);
      const expirationDate = new Date(decodedToken.exp * 1000);
      console.debug("Refresh Token will expire on: ", expirationDate);
      localStorage.setItem("RefreshToken", data.refresh_token);
    }
    if (data.access_token) {
      const decodedToken = jwtDecode<JwtDecodeExp>(data.access_token!);
      const expirationDate = new Date(decodedToken.exp * 1000);
      console.debug("Token will expire on: ", expirationDate);
      localStorage.setItem("AccessToken", data.access_token);
    }
  }

  // stop the refresh cycle on cleanup
  static stopRefreshing(): void {
    if (this.refreshIntervalId !== undefined) {
      console.log("Clearing refresh interval");
      clearInterval(this.refreshIntervalId);
      this.refreshIntervalId = undefined;
      this.hasStartedRefreshInterval = false;
    }
  }
}
