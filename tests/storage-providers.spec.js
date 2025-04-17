// @ts-check
import { test, expect } from '@playwright/test';
import { login, createStorageProvider, cleanupCreatedProviders, testProviderConnection } from './test-setup.js';

/**
 * This test logs in to the application and tests storage providers
 */
test.describe('Storage Providers', () => {
  // Login before each test
  test.beforeEach(async ({ page }) => {
    await login(
      page, 
      process.env.TEST_USERNAME || 'admin@example.com', 
      process.env.TEST_PASSWORD || 'admin'
    );
  });
  
  // Clean up created providers after tests
  test.afterEach(async ({ page }) => {
    await cleanupCreatedProviders(page);
  });
  
  // Test navigating to storage providers page
  test('can navigate to storage providers page', async ({ page }) => {
    // Navigate to storage providers page
    await page.goto('/storage-providers');
    
    // Verify the page title
    await expect(page.locator('h1:has-text("Storage Providers")')).toBeVisible();
  });
  
  // Test creating a new storage provider (SFTP)
  test('can create a new SFTP storage provider', async ({ page }) => {
    // Navigate to the storage providers page
    await page.goto('/storage-providers');
    
    // Click on "New Provider" button
    await page.click('a:has-text("New Provider")');
    
    // Verify we're on the provider creation form
    await expect(page.locator('h1:has-text("New Storage Provider")')).toBeVisible();
    
    // Fill in the form for SFTP provider
    await page.fill('#name', 'Test SFTP Provider');
    await page.selectOption('#type', 'sftp');
    
    // Wait for the SFTP fields to be visible
    await expect(page.locator('#sftp-ftp-fields')).toBeVisible();
    
    // Fill the SFTP form fields
    await page.fill('#host', process.env.SFTP_HOST || 'sftp.example.com');
    await page.fill('#port', process.env.SFTP_PORT || '22');
    await page.fill('#username', process.env.SFTP_USERNAME || 'testuser');
    await page.fill('#password', process.env.SFTP_PASSWORD || 'testpassword');
    if (process.env.SFTP_KEY_FILE) {
      await page.fill('#keyFile', process.env.SFTP_KEY_FILE);
    }
    
    // Save the provider
    await page.click('button:has-text("Save Provider")');
    
    // Verify we're redirected back to the providers list
    await expect(page.locator('h1:has-text("Storage Providers")')).toBeVisible();
    
    // Verify our new provider is in the list - using more specific selector
    await expect(page.locator('.text-blue-600.truncate:has-text("Test SFTP Provider")')).toBeVisible();
    
    // Optionally test the connection if not using stubs
    if (process.env.TEST_USE_STUBBED_PROVIDERS !== 'true') {
      await testProviderConnection(page, 'Test SFTP Provider');
    }
  });
  
  // Test creating a new FTP storage provider
  test('can create a new FTP storage provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test FTP Provider',
      type: 'ftp',
      host: process.env.FTP_HOST || 'ftp.example.com',
      port: process.env.FTP_PORT || '21',
      username: process.env.FTP_USERNAME || 'ftpuser',
      password: process.env.FTP_PASSWORD || 'ftppassword'
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Optionally test the connection if not using stubs
    if (process.env.TEST_USE_STUBBED_PROVIDERS !== 'true') {
      await testProviderConnection(page, providerName);
    }
  });
  
  // Test creating a new Hetzner storage provider
  test('can create a new Hetzner storage provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test Hetzner Provider',
      type: 'hetzner',
      host: process.env.HETZNER_HOST || 'u123456.your-storagebox.de',
      port: process.env.HETZNER_PORT || '23',
      username: process.env.HETZNER_USERNAME || 'u123456',
      password: process.env.HETZNER_PASSWORD || 'hetznerpassword'
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Optionally test the connection if not using stubs
    if (process.env.TEST_USE_STUBBED_PROVIDERS !== 'true') {
      await testProviderConnection(page, providerName);
    }
  });
  
  // Test creating a new SMB/CIFS storage provider
  test('can create a new SMB/CIFS storage provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test SMB Provider',
      type: 'smb',
      host: process.env.SMB_HOST || 'fileserver.example.com',
      port: process.env.SMB_PORT || '445',
      username: process.env.SMB_USERNAME || 'smbuser',
      password: process.env.SMB_PASSWORD || 'smbpassword',
      share: process.env.SMB_SHARE || 'Shared',
      domain: process.env.SMB_DOMAIN || 'WORKGROUP'
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Optionally test the connection if not using stubs
    if (process.env.TEST_USE_STUBBED_PROVIDERS !== 'true') {
      await testProviderConnection(page, providerName);
    }
  });
  
  // Test creating a new S3 storage provider
  test('can create a new S3 storage provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test S3 Provider',
      type: 's3',
      endpoint: process.env.S3_ENDPOINT || 's3.amazonaws.com',
      region: process.env.S3_REGION || 'us-east-1',
      bucket: process.env.S3_BUCKET || 'test-bucket',
      accessKey: process.env.S3_ACCESS_KEY || 'AKIAIOSFODNN7EXAMPLE',
      secretKey: process.env.S3_SECRET_KEY || 'wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY'
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Optionally test the connection if not using stubs
    if (process.env.TEST_USE_STUBBED_PROVIDERS !== 'true') {
      await testProviderConnection(page, providerName);
    }
  });
  
  // Test creating a new Wasabi storage provider
  test('can create a new Wasabi storage provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test Wasabi Provider',
      type: 'wasabi',
      endpoint: process.env.WASABI_ENDPOINT || 's3.wasabisys.com',
      region: process.env.WASABI_REGION || 'us-east-1',
      bucket: process.env.WASABI_BUCKET || 'wasabi-test-bucket',
      accessKey: process.env.WASABI_ACCESS_KEY || 'WASABIEXAMPLEKEY',
      secretKey: process.env.WASABI_SECRET_KEY || 'wasabiexamplesecretkey12345'
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Optionally test the connection if not using stubs
    if (process.env.TEST_USE_STUBBED_PROVIDERS !== 'true') {
      await testProviderConnection(page, providerName);
    }
  });
  
  // Test creating a new MinIO storage provider
  test('can create a new MinIO storage provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test MinIO Provider',
      type: 'minio',
      endpoint: process.env.MINIO_ENDPOINT || 'play.min.io',
      region: process.env.MINIO_REGION || 'us-east-1',
      bucket: process.env.MINIO_BUCKET || 'minio-test-bucket',
      accessKey: process.env.MINIO_ACCESS_KEY || 'Q3AM3UQ867SPQQA43P2F',
      secretKey: process.env.MINIO_SECRET_KEY || 'zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG'
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Optionally test the connection if not using stubs
    if (process.env.TEST_USE_STUBBED_PROVIDERS !== 'true') {
      await testProviderConnection(page, providerName);
    }
  });
  
  // Test creating a new Backblaze B2 storage provider
  test('can create a new Backblaze B2 storage provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test B2 Provider',
      type: 'b2',
      endpoint: process.env.B2_ENDPOINT || 's3.us-west-002.backblazeb2.com',
      region: process.env.B2_REGION || 'us-west-002',
      bucket: process.env.B2_BUCKET || 'b2-test-bucket',
      accessKey: process.env.B2_ACCESS_KEY || 'B2EXAMPLEKEYID',
      secretKey: process.env.B2_SECRET_KEY || 'b2examplesecretkeyvalueforbackblazeb2'
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Optionally test the connection if not using stubs
    if (process.env.TEST_USE_STUBBED_PROVIDERS !== 'true') {
      await testProviderConnection(page, providerName);
    }
  });
  
  // Test creating a new WebDAV storage provider
  test('can create a new WebDAV storage provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test WebDAV Provider',
      type: 'webdav',
      host: process.env.WEBDAV_HOST || 'webdav.example.com',
      port: process.env.WEBDAV_PORT || '443',
      username: process.env.WEBDAV_USERNAME || 'webdavuser',
      password: process.env.WEBDAV_PASSWORD || 'webdavpassword'
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Optionally test the connection if not using stubs
    if (process.env.TEST_USE_STUBBED_PROVIDERS !== 'true') {
      await testProviderConnection(page, providerName);
    }
  });
  
  // Test creating a new Nextcloud storage provider
  test('can create a new Nextcloud storage provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test Nextcloud Provider',
      type: 'nextcloud',
      host: process.env.NEXTCLOUD_HOST || 'nextcloud.example.com',
      port: process.env.NEXTCLOUD_PORT || '443',
      username: process.env.NEXTCLOUD_USERNAME || 'nextclouduser',
      password: process.env.NEXTCLOUD_PASSWORD || 'nextcloudpassword'
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Optionally test the connection if not using stubs
    if (process.env.TEST_USE_STUBBED_PROVIDERS !== 'true') {
      await testProviderConnection(page, providerName);
    }
  });
  
  // Test creating a new Google Drive provider
  test('can create a new Google Drive provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test Google Drive Provider',
      type: 'gdrive',
      clientID: process.env.GDRIVE_CLIENT_ID || '1234567890-abcdefghijklmnopqrstuvwxyz.apps.googleusercontent.com',
      clientSecret: process.env.GDRIVE_CLIENT_SECRET || 'GOCSPX-abcdefghijklmnopqrstuvwxyz',
      driveID: process.env.GDRIVE_DRIVE_ID,
      teamDrive: process.env.GDRIVE_TEAM_DRIVE
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Note: We don't test connection for cloud providers as they require OAuth authentication
  });
  
  // Test creating a new Google Photos provider
  test('can create a new Google Photos provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test Google Photos Provider',
      type: 'gphotos',
      clientID: process.env.GPHOTOS_CLIENT_ID || '1234567890-abcdefghijklmnopqrstuvwxyz.apps.googleusercontent.com',
      clientSecret: process.env.GPHOTOS_CLIENT_SECRET || 'GOCSPX-abcdefghijklmnopqrstuvwxyz'
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Note: We don't test connection for cloud providers as they require OAuth authentication
  });
  
  // Test creating a new OneDrive provider
  test('can create a new OneDrive provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test OneDrive Provider',
      type: 'onedrive',
      clientID: process.env.ONEDRIVE_CLIENT_ID || '12345678-1234-1234-1234-123456789012',
      clientSecret: process.env.ONEDRIVE_CLIENT_SECRET || 'abc~12345678901234567890abcdefghijklmn'
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Note: We don't test connection for cloud providers as they require OAuth authentication
  });
  
  // Test creating a local file system provider
  test('can create a new local filesystem provider', async ({ page }) => {
    // Create a provider using the helper function
    const providerName = await createStorageProvider(page, {
      name: 'Test Local Provider',
      type: 'local',
      localPath: process.env.LOCAL_PATH || '/tmp/test-storage'
    });
    
    // Verify our new provider is in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Optionally test the connection if not using stubs
    if (process.env.TEST_USE_STUBBED_PROVIDERS !== 'true') {
      await testProviderConnection(page, providerName);
    }
  });
  
  // Test editing a storage provider
  test('can edit an existing storage provider', async ({ page }) => {
    // Create a provider first
    const providerName = await createStorageProvider(page, {
      name: 'Provider To Edit',
      type: 'sftp',
      host: 'original.example.com',
      port: '22',
      username: 'original',
      password: 'password'
    });
    
    // Find and click the edit button for this provider
    const providerRow = page.locator(`li:has(.text-blue-600.truncate:has-text("${providerName}"))`);
    await providerRow.locator('a:has-text("Edit")').click();
    
    // Verify we're on the edit page
    await expect(page.locator('h1:has-text("Edit Storage Provider")')).toBeVisible();
    
    // Change the name and host
    await page.fill('#name', 'Edited Provider');
    await page.fill('#host', 'edited.example.com');
    
    // Save the changes
    await page.click('button:has-text("Save Provider")');
    
    // Verify we're back at the list
    await expect(page.locator('h1:has-text("Storage Providers")')).toBeVisible();
    
    // Verify the updated provider name is showing
    await expect(page.locator('.text-blue-600.truncate:has-text("Edited Provider")')).toBeVisible();
    await expect(page.locator('li:has(.text-blue-600.truncate:has-text("Edited Provider"))').locator('text=edited.example.com')).toBeVisible();
  });
  
  // Test testing a storage provider connection
  test('can test a storage provider connection', async ({ page }) => {
    // Create a provider to test
    const providerName = await createStorageProvider(page, {
      name: 'Provider To Test Connection',
      type: 'local',
      localPath: '/tmp/test-connection'
    });
    
    // Test the connection
    await testProviderConnection(page, providerName);
    
    // Toast should be visible (we don't assert success because it depends on actual connection ability)
    await expect(page.locator('.toast')).toBeVisible();
  });
  
  // Test duplicating a storage provider
  test('can duplicate a storage provider', async ({ page }) => {
    // Create a provider to duplicate
    const providerName = await createStorageProvider(page, {
      name: 'Provider To Duplicate',
      type: 'local',
      localPath: '/tmp/duplicate-test'
    });
    
    // Navigate to storage providers
    await page.goto('/storage-providers');
    
    // Find the provider to duplicate
    const providerRow = page.locator(`li:has(.text-blue-600.truncate:has-text("${providerName}"))`);
    
    // Click the duplicate button
    await providerRow.locator('button:has-text("Duplicate")').click();
    
    // There should now be a provider with the same name in the list more than once
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`).count()).toBeGreaterThan(1);
  });
  
  // Test deleting a storage provider
  test('can delete a storage provider', async ({ page }) => {
    // Create a provider to delete
    const providerName = await createStorageProvider(page, {
      name: 'Provider To Delete',
      type: 'local',
      localPath: '/tmp/delete-me'
    });
    
    // Navigate back to the providers list to refresh
    await page.goto('/storage-providers');
    
    // Verify the provider was created
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).toBeVisible();
    
    // Find and click the delete button for this provider
    const providerRow = page.locator(`li:has(.text-blue-600.truncate:has-text("${providerName}"))`);
    await providerRow.locator('button:has-text("Delete"):not([hx-delete])').click();
    
    // A confirmation dialog should appear - confirm deletion
    await page.locator('button[hx-delete]:has-text("Delete"):visible').click();
    
    // Verify the provider is no longer in the list
    await expect(page.locator(`.text-blue-600.truncate:has-text("${providerName}")`)).not.toBeVisible();
  });
}); 