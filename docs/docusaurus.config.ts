import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const config: Config = {
  title: 'GoMFT',
  tagline: 'A modern, web-based managed file transfer solution',
  favicon: 'favicon.ico',

  // Set the production url of your site here
  url: 'https://starfleetcptn.github.io',
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
  baseUrl: '/GoMFT/',

  // GitHub pages deployment config.
  // If you aren't using GitHub pages, you don't need these.
  organizationName: 'StarFleetCPTN', // Usually your GitHub org/user name.
  projectName: 'GoMFT', // Usually your repo name.
  deploymentBranch: 'gh-pages',
  trailingSlash: false,

  onBrokenLinks: 'warn',
  onBrokenMarkdownLinks: 'warn',

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang. For example, if your site is Chinese, you
  // may want to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          // Please change this to your repo.
          editUrl:
            'https://github.com/StarFleetCPTN/GoMFT/tree/main/documentation',
        },
        blog: {
          showReadingTime: true,
          // Please change this to your repo.
          editUrl:
            'https://github.com/StarFleetCPTN/GoMFT/tree/main/documentation',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  // Add the local search plugin
  plugins: [
    [
      require.resolve('@easyops-cn/docusaurus-search-local'),
      {
        // Whether to also index the docs/blog not written in the current language (false by default)
        indexDocs: true,
        indexBlog: true,
        // Whether to also add the language name to the document ID, to differentiate documents with the same ID but different languages (false by default)
        language: ['en'],
        // Optional: path to a file containing a list of words to be highlighted in the search results (empty by default)
        highlightSearchTermsOnTargetPage: true,
      },
    ],
  ],

  themeConfig: {
    // Replace with your project's social card
    image: 'img/gomft-social-card.jpg',
    navbar: {
      title: 'GoMFT',
      logo: {
        alt: 'GoMFT Logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Documentation',
        },
        {to: '/docs/introduction/overview', label: 'Getting Started', position: 'left'},
        {to: '/docs/development/contributing', label: 'Contributing', position: 'left'},
        {
          href: 'https://github.com/StarFleetCPTN/GoMFT',
          label: 'GitHub',
          position: 'right',
        },
        {
          href: 'https://discord.gg/f9dwtM3j',
          className: 'header-discord-link',
          'aria-label': 'Discord community',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {
              label: 'Introduction',
              to: '/docs/introduction/overview',
            },
            {
              label: 'Installation',
              to: '/docs/getting-started/installation',
            },
            {
              label: 'Features',
              to: '/docs/introduction/features',
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'Discord',
              href: 'https://discord.gg/f9dwtM3j',
            },
            {
              label: 'GitHub Discussions',
              href: 'https://github.com/StarFleetCPTN/GoMFT/discussions',
            },
            {
              label: 'Issues',
              href: 'https://github.com/StarFleetCPTN/GoMFT/issues',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/StarFleetCPTN/GoMFT',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} GoMFT. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
