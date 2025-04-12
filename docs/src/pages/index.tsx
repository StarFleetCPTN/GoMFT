import React from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';

import styles from './index.module.css';

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <div className="row">
          <div className="col col--8 col--offset-2">
            <img 
              className="hero__logo"
              src="img/logo.svg"
              alt="Project Logo"
            />
            <Heading as="h1" className="hero__title">
              {siteConfig.title}
            </Heading>
            <p className="hero__subtitle">{siteConfig.tagline}</p>
            <div className={styles.buttons}>
              <Link
                className="button button--secondary button--lg"
                to="/docs/introduction/overview">
                Get Started
              </Link>
              <Link
                className="button button--outline button--lg button--secondary"
                to="https://github.com/StarFleetCPTN/GoMFT">
                GitHub
              </Link>
            </div>
          </div>
        </div>
      </div>
    </header>
  );
}

function FeatureList() {
  return [
    {
      title: 'Easy to Use',
      description: (
        <>
          GoMFT was designed from the ground up to be easily installed and
          used to get your file transfers up and running quickly.
        </>
      ),
    },
    {
      title: 'Multi-Protocol Support',
      description: (
        <>
          GoMFT leverages the power of rclone to support over 40 storage
          systems including S3, SFTP, Google Drive, and more.
        </>
      ),
    },
    {
      title: 'Powerful Scheduling',
      description: (
        <>
          Schedule your file transfers using familiar cron syntax for recurring transfers,
          or run them on-demand with the intuitive web interface.
        </>
      ),
    },
  ];
}

interface FeatureProps {
  title: string;
  description: React.ReactElement;
}

function Feature({title, description}: FeatureProps) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center padding-horiz--md">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function Home(): React.ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={`${siteConfig.title} - Modern Managed File Transfer Solution`}
      description="GoMFT is a modern, open-source managed file transfer solution with multi-protocol support, scheduling capabilities, and a user-friendly web interface">
      <HomepageHeader />
      <main>
        <section className={styles.features}>
          <div className="container">
            <div className="row">
              {FeatureList().map((props, idx) => (
                <Feature key={idx} {...props} />
              ))}
            </div>
          </div>
        </section>

        <section className={clsx(styles.section, styles.sectionAlt)}>
          <div className="container">
            <div className="row">
              <div className="col col--6">
                <Heading as="h2">Modern Web Interface</Heading>
                <p>
                  GoMFT provides a clean, responsive web interface for managing your file transfers.
                  The dashboard gives you at-a-glance information about transfer status, recent jobs,
                  and system health.
                </p>
                <Link
                  className="button button--primary"
                  to="/docs/core-concepts/monitoring">
                  Learn More
                </Link>
              </div>
              <div className="col col--6">
                <img src="img/dashboard.gomft.png" alt="GoMFT Dashboard" className="shadow--md" style={{borderRadius: '8px'}} />
              </div>
            </div>
          </div>
        </section>

        <section className={styles.section}>
          <div className="container">
            <div className="row">
              <div className="col col--6">
                <img src="img/transfer.config.gomft.png" alt="Transfer Configuration" className="shadow--md" style={{borderRadius: '8px'}} />
              </div>
              <div className="col col--6">
                <Heading as="h2">Easy Deployment</Heading>
                <p>
                  Deploy GoMFT quickly using Docker, or install it directly on your system.
                  The application is lightweight and can run on various platforms including
                  Linux, macOS, and Windows.
                </p>
                <Link
                  className="button button--primary"
                  to="/docs/getting-started/installation">
                  Installation Guide
                </Link>
              </div>
            </div>
          </div>
        </section>

        <section className={clsx(styles.section, styles.sectionAlt)}>
          <div className="container">
            <div className="text--center">
              <Heading as="h2">Ready to Get Started?</Heading>
              <p>
                Check out our documentation to learn how to set up and use GoMFT for your file transfer needs.
              </p>
              <div className={styles.buttons}>
                <Link
                  className="button button--primary button--lg"
                  to="/docs/introduction/overview">
                  Read the Docs
                </Link>
                <Link
                  className="button button--secondary button--lg"
                  to="https://github.com/StarFleetCPTN/GoMFT">
                  GitHub Repository
                </Link>
              </div>
            </div>
          </div>
        </section>
      </main>
    </Layout>
  );
}
