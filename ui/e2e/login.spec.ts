import { test, expect } from '@playwright/test';

test.describe('Authentication Flow', () => {
  test('unauthenticated users are redirected to login', async ({ page }) => {
    await page.goto('/projects');
    
    // Should be redirected to /login
    await expect(page).toHaveURL(/.*\/login/);
    
    // Should see the login form
    await expect(page.getByRole('heading', { name: /Welcome back to/i })).toBeVisible();
    await expect(page.getByPlaceholder('admin@example.com')).toBeVisible();
  });

  test('user can login with valid credentials', async ({ page }) => {
    await page.goto('/login');
    
    // We are testing UI logic using the mock responses until the real DB is seeded
    // In a real e2e environment, we would seed the database and use real credentials.
    await page.getByPlaceholder('admin@example.com').fill('admin');
    await page.getByPlaceholder('••••••••').fill('password');
    
    // Mock the /api/auth/login response to succeed
    await page.route('**/api/auth/login', async route => {
      const json = {
        success: true,
        data: {
          id: 1,
          username: "admin",
          email: "admin@example.com",
          global_role: "admin"
        }
      };
      await route.fulfill({ json });
    });

    // Mock the /api/auth/me response
    await page.route('**/api/auth/me', async route => {
      const json = {
        success: true,
        data: {
          id: 1,
          username: "admin",
          email: "admin@example.com",
          global_role: "admin"
        }
      };
      await route.fulfill({ json });
    });

    // Mock the /api/projects response
    await page.route('**/api/projects', async route => {
      const json = {
        success: true,
        data: []
      };
      await route.fulfill({ json });
    });

    await page.getByRole('button', { name: /Sign In/i }).click();

    // Should redirect to projects page
    await expect(page).toHaveURL(/.*\/projects/);
    await expect(page.getByRole('heading', { name: 'Projects' })).toBeVisible();
  });
});
