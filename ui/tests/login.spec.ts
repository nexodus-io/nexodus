import { test, expect } from "@playwright/test";
import lib from "./env";

test("Login takes you to the Dashboard", async ({ page }) => {
  await page.goto("/#/login");
  await page.getByRole("button", { name: "Login" }).click();
  await page.getByLabel("Username or email").fill(lib.username);
  await page.getByLabel("Password").fill(lib.password);
  await page.getByLabel("Password").press("Enter");
  // Validate Dashboard
  await expect(page.getByText("Welcome to Nexodus")).toBeVisible();
  await page.getByRole("menuitem", { name: "Dashboard" }).click();
  await expect(page.getByText("Welcome to Nexodus")).toBeVisible();
  // Validate Organizations
  await page.getByRole("menuitem", { name: "Organizations" }).click();
  await expect(
    page.locator("#react-admin-title").getByText("Organizations"),
  ).toBeVisible();
  // Validate VPCs
  await page.getByRole("menuitem", { name: "VPCs" }).click();
  await expect(page.getByText("default vpc")).toBeVisible();
  await page.getByRole("cell", { name: "default vpc" }).click();
  await expect(page.getByText("100.64.0.0")).toBeVisible();
  // Validate Devices
  await page.getByRole("menuitem", { name: "Devices" }).click();
  await expect(
    page.locator("#react-admin-title").getByText("Devices"),
  ).toBeVisible();
  // Validate Sites
  await page.getByRole("menuitem", { name: "Sites" }).click();
  // Validate Invitations
  await page.getByRole("menuitem", { name: "Invitations" }).click();
  await expect(page.getByLabel("Create")).toBeVisible();
  await page.getByLabel("Create").click();
  await expect(page.getByLabel("Email Address *")).toBeVisible();
  // Validate Security Groups
  await page.getByRole("menuitem", { name: "Security Groups" }).click();
  await page.getByRole("cell", { name: "default vpc security group" }).click();
  await page.getByRole("button", { name: "Edit Rules" }).click();
  await page.getByRole("button", { name: "Add Rule" }).click();
  await page.getByRole("combobox").first().click();
  await page.getByRole("option", { name: "All ICMP" }).click();
  await page.getByRole("button", { name: "Save Rules" }).click();
  await page.getByRole("tab", { name: "Outbound Rules" }).click();
  await page.getByRole("button", { name: "Add Rule" }).click();
  await page.getByRole("combobox").first().click();
  await page.getByRole("option", { name: "All ICMP" }).click();
  await page.getByRole("button", { name: "Save Rules" }).click();
  await page.getByRole("tab", { name: "Inbound Rules" }).click();
  await expect(page.getByText("All ICMP")).toBeVisible();
  await page.getByRole("tab", { name: "Outbound Rules" }).click();
  await expect(page.getByText("All ICMP")).toBeVisible();
  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("tab", { name: "Inbound Rules" }).click();
  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("button", { name: "Save Rules" }).click();
  // Validate Registration Keys
  await page.getByRole("menuitem", { name: "Registration Keys" }).click();
  await page.getByLabel("Create").click();
  await expect(page.getByLabel("Description")).toBeVisible();
  // Validate Logout
  await page.getByLabel("Profile").click();
  await page.getByText("Logout").click();
  await expect(page.getByRole("button", { name: "Login" })).toBeVisible();
});
