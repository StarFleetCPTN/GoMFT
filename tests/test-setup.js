// @ts-check
import { expect } from '@playwright/test';
import dotenv from 'dotenv';
import fs from 'fs';
import path from 'path';

// Load environment variables from .env.test if it exists
const testEnvPath = path.join(process.cwd(), '.env.test');
if (fs.existsSync(testEnvPath)) {
  dotenv.config({ path: testEnvPath });
} else {
  // Fallback to regular .env
  dotenv.config();
}

// Store providers created during the test so we can clean them up
const createdProviders = new Set();

/**
 * Custom expect extension to wait for toast notifications
 */
expect.extend({
  async toShowToast(page, type, message) {
    const toastSelector = type ? `.toast-${type}` : '.toast';
    
    try {
      await page.waitForSelector(toastSelector, { timeout: 5000 });
      
      if (message) {
        const toastContent = await page.locator(toastSelector).textContent();
        return {
          pass: toastContent?.includes(message),
          message: () => `Expected toast to contain "${message}" but found "${toastContent}"`
        };
      }
      
      return {
        pass: true,
        message: () => `Toast of type ${type || 'any'} was found`
      };
    } catch (e) {
      return {
        pass: false,
        message: () => `Toast of type ${type || 'any'} was not found: ${e.message}`
      };
    }
  }
});

/**
 * Helper to log in a user
 * @param {import('@playwright/test').Page} page
 * @param {string} username
 * @param {string} password
 */
export async function login(page, username, password) {
  // Navigate to the login page
  await page.goto('/login');
  
  // Check that we're on the login page
  await expect(page.locator('h2:has-text("Sign In")')).toBeVisible();
  
  // Fill in login form
  await page.fill('#email', username);
  await page.fill('#password', password);
  
  // Submit the form
  await page.click('button:has-text("Sign in")');
  
  // Wait for navigation to complete (dashboard should load)
  await page.waitForURL('**/dashboard');
  
  // Verify we're logged in by checking for user menu
  await expect(page.locator('#user-menu-button')).toBeVisible();
}

/**
 * Helper to create a storage provider for testing
 * @param {import('@playwright/test').Page} page
 * @param {Object} providerData
 * @returns {Promise<string>} The name of the created provider
 */
export async function createStorageProvider(page, providerData) {
  // Navigate to the storage providers page
  await page.goto('/storage-providers');
  
  // Click on "New Provider" button
  await page.click('a:has-text("New Provider")');
  
  // Fill in the common fields
  await page.fill('#name', providerData.name);
  await page.selectOption('#type', providerData.type);
  
  // Fill in type-specific fields
  switch (providerData.type) {
    case 'sftp':
    case 'ftp':
    case 'hetzner':
      await page.fill('#host', providerData.host || 'test.example.com');
      if (providerData.port) await page.fill('#port', providerData.port);
      await page.fill('#username', providerData.username || 'testuser');
      await page.fill('#password', providerData.password || 'testpass');
      break;
      
    case 'smb':
      await page.fill('#host', providerData.host || 'fileserver.example.com');
      if (providerData.port) await page.fill('#port', providerData.port);
      await page.fill('#username', providerData.username || 'smbuser');
      await page.fill('#password', providerData.password || 'smbpassword');
      await page.fill('#share', providerData.share || 'Shared');
      if (providerData.domain) await page.fill('#domain', providerData.domain);
      break;
      
    case 's3':
    case 'wasabi':
    case 'minio':
      await page.fill('#endpoint', providerData.endpoint || 's3.example.com');
      await page.fill('#region', providerData.region || 'us-east-1');
      await page.fill('#bucket', providerData.bucket || 'test-bucket');
      await page.fill('#accessKey', providerData.accessKey || 'AKIATEST');
      await page.fill('#secretKey', providerData.secretKey || 'secretkey123');
      break;
      
    case 'gdrive':
    case 'gphotos':
    case 'onedrive':
      await page.fill('#clientID', providerData.clientID || '123456789012-abcdef.apps.googleusercontent.com');
      await page.fill('#clientSecret', providerData.clientSecret || 'GOCSPX-abcdefghij');
      break;
      
    case 'local':
      await page.fill('#localPath', providerData.localPath || '/tmp/test-path');
      break;
  }
  
  // Save the provider
  await page.click('button:has-text("Save Provider")');
  
  // Wait to be redirected back to the list
  await page.waitForURL('**/storage-providers*');
  
  // Add to the list of created providers for cleanup
  createdProviders.add(providerData.name);
  
  return providerData.name;
}

/**
 * Helper to delete a storage provider by name
 * @param {import('@playwright/test').Page} page
 * @param {string} providerName Name of the provider to delete
 */
export async function deleteStorageProvider(page, providerName) {
  // Navigate to the storage providers page
  await page.goto('/storage-providers');
  
  // Look for the provider with the given name
  const providerRow = page.locator(`li:has(.text-blue-600.truncate:has-text("${providerName}"))`);
  
  // Check if the provider exists
  if (await providerRow.count() === 0) {
    return; // Provider doesn't exist or was already deleted
  }
  
  // Click the delete button - Using a more specific selector to get only the visible button with the trash icon
  await providerRow.locator('button:has-text("Delete"):not([hx-delete])').click();
  
  // Confirm deletion in the dialog - use a more specific selector for the confirmation button
  await page.locator('button[hx-delete]:has-text("Delete"):visible').click();
  
  // Wait for the toast notification
  try {
    await page.waitForSelector('.toast', { timeout: 5000 });
  } catch (e) {
    // Continue even if toast doesn't appear
  }
  
  // Remove from the set of created providers
  createdProviders.delete(providerName);
}

/**
 * Cleanup all created providers
 * @param {import('@playwright/test').Page} page
 */
export async function cleanupCreatedProviders(page) {
  // Only attempt cleanup if there are providers to clean up
  if (createdProviders.size === 0) return;
  
  // Navigate to the storage providers page
  await page.goto('/storage-providers');
  
  // Delete each created provider
  for (const providerName of createdProviders) {
    await deleteStorageProvider(page, providerName);
  }
  
  // Clear the set
  createdProviders.clear();
}

/**
 * Helper to test a provider connection
 * @param {import('@playwright/test').Page} page
 * @param {string} providerName Name of the provider to test
 */
export async function testProviderConnection(page, providerName) {
  // Navigate to the storage providers page
  await page.goto('/storage-providers');
  
  // Find the provider row
  const providerRow = page.locator(`li:has(.text-blue-600.truncate:has-text("${providerName}"))`);
  
  // Click the test button
  await providerRow.locator('button:has-text("Test")').click();
  
  // Wait for toast notification
  await page.waitForSelector('.toast', { timeout: 10000 });
  
  // Return whether it was successful (toast-success) or not
  return await page.locator('.toast-success').count() > 0;
} 