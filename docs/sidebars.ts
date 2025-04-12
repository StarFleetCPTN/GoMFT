import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

/**
 * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */
const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'category',
      label: 'Introduction',
      items: ['introduction/overview', 'introduction/features'],
    },
    {
      type: 'category',
      label: 'Getting Started',
      items: ['getting-started/installation', 'getting-started/configuration', 'getting-started/quick-start', 'getting-started/docker', 'getting-started/traditional'],
    },
    {
      type: 'category',
      label: 'Core Concepts',
      items: ['core-concepts/transfers', 'core-concepts/connections', 'core-concepts/schedules', 'core-concepts/monitoring'],
    },
    {
      type: 'category',
      label: 'Advanced Features',
      items: [
        'advanced/notifications-overview',
        'advanced/gotify-notifications',
        'advanced/ntfy-notifications',
        'advanced/pushbullet-notifications',
        'advanced/pushover-notifications',
        'advanced/webhook-notifications',
        'advanced/admin-tools'
      ],
    },
    {
      type: 'category',
      label: 'Security',
      items: ['security/best-practices', 'security/authentication', 'security/non-root'],
    },
    {
      type: 'category',
      label: 'Development',
      items: ['development/project-structure', 'development/contributing'],
    },
  ],
};

export default sidebars;
