import { jwtDecode } from "jwt-decode";

export class RefreshManager {
  static refreshIntervalId: number | undefined = undefined;
  static REFRESH_INTERVAL_MS = 90 * 1000;

  // one-time refresh POST
  static async postRefresh(api: string): Promise<void> {
    const request = new Request(`${api}/web/refresh`, {
      method: "POST",
      credentials: "include",
    });

    try {
      const response = await fetch(request);
      if (!response.ok) {
        console.error(
          `Received ${response.status} from server during refresh.`,
        );
        if (
          response.headers.get("Content-Type")?.includes("application/json")
        ) {
          const errorData = await response.json();
          console.error("Error data from server:", errorData);
        }
      }
    } catch (error) {
      console.error("An error occurred during token fetch:", error);
    }
  }

  // long-running refreshing on a timer
  static async startRefreshing(api: string): Promise<void> {
    console.debug("startRefreshing called.");
    if (this.refreshIntervalId === undefined) {
      console.debug("Starting the refresh interval.");
      this.refreshIntervalId = window.setInterval(async () => {
        try {
          console.debug("Start Refreshing Firing.");
          await this.postRefresh(api);
          console.debug("refreshed successfully.");
        } catch (e) {
          console.error("Error in refresh:", e);
          this.stopRefreshing();
        }
      }, RefreshManager.REFRESH_INTERVAL_MS);
    }
  }

  // stop the refresh cycle on cleanup
  static stopRefreshing(): void {
    if (this.refreshIntervalId !== undefined) {
      console.log("Clearing refresh interval");
      window.clearInterval(this.refreshIntervalId);
      this.refreshIntervalId = undefined;
    }
  }
}
