import { test, expect } from '@playwright/test';
import lib from "./env";

test('Login takes you to the Dashboard', async ({ page }) => {
await page.goto('/#/login');
await page.getByRole('button', { name: 'Login' }).click();
await page.getByLabel('Username or email').fill(lib.username);
await page.getByLabel('Password').fill(lib.password);
await page.getByLabel('Password').press('Enter');

await expect(page.getByText('Welcome to Nexodus')).toBeVisible();


});