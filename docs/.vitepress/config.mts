import { defineConfig } from 'vitepress';
import { withMermaid } from 'vitepress-plugin-mermaid';

const base = process.env.DOCS_BASE || '/';
const siteOrigin = (process.env.DOCS_ORIGIN || 'https://azayr.github.io/dokyr').replace(/\/$/, '');
const repository = 'https://github.com/azayr/dokyr';
const description = 'An open-source, single-server PaaS for deploying containers, routing domains, and operating your own infrastructure.';

function pageUrl(relativePath: string) {
  if (relativePath === 'index.md') return `${siteOrigin}/`;
  return `${siteOrigin}/${relativePath.replace(/(^|\/)index\.md$/, '$1').replace(/\.md$/, '')}`;
}

export default withMermaid(defineConfig({
  lang: 'en-US',
  title: 'Dokyr',
  titleTemplate: ':title · Dokyr',
  description,
  base,
  appearance: true,
  cleanUrls: true,
  lastUpdated: true,
  mermaid: {
    securityLevel: 'strict',
    theme: 'base',
    themeVariables: {
      fontFamily: 'Inter, ui-sans-serif, system-ui, sans-serif',
      primaryColor: '#eaf2ff',
      primaryTextColor: '#111827',
      primaryBorderColor: '#0d63e5',
      lineColor: '#58708f',
      secondaryColor: '#f7f9fc',
      tertiaryColor: '#ffffff'
    }
  },
  sitemap: {
    hostname: `${siteOrigin}/`
  },
  head: [
    ['meta', { name: 'theme-color', media: '(prefers-color-scheme: light)', content: '#f7f9fc' }],
    ['meta', { name: 'theme-color', media: '(prefers-color-scheme: dark)', content: '#0b0e14' }],
    ['meta', { name: 'author', content: 'Dokyr contributors' }],
    ['meta', { name: 'robots', content: 'index, follow, max-image-preview:large' }],
    ['meta', { property: 'og:site_name', content: 'Dokyr' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:locale', content: 'en_US' }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
    ['link', { rel: 'icon', type: 'image/svg+xml', href: `${base}logo.svg` }],
    ['link', { rel: 'alternate', type: 'text/plain', href: `${base}llms.txt`, title: 'Dokyr documentation for language models' }],
    [
      'script',
      { type: 'application/ld+json' },
      JSON.stringify({
        '@context': 'https://schema.org',
        '@graph': [
          {
            '@type': 'WebSite',
            name: 'Dokyr',
            url: `${siteOrigin}/`,
            description,
            inLanguage: 'en-US'
          },
          {
            '@type': 'SoftwareApplication',
            name: 'Dokyr',
            applicationCategory: 'DeveloperApplication',
            operatingSystem: 'Linux',
            url: `${siteOrigin}/`,
            codeRepository: repository,
            downloadUrl: `${repository}/pkgs/container/dokyr`,
            license: `${repository}/blob/main/LICENSE`,
            description,
            softwareVersion: '0.2.32',
            offers: {
              '@type': 'Offer',
              price: '0',
              priceCurrency: 'USD'
            }
          }
        ]
      })
    ]
  ],
  transformHead({ pageData }) {
    const url = pageUrl(pageData.relativePath);
    const pageDescription = pageData.frontmatter.description || description;
    const pageTitle = pageData.frontmatter.title ? `${pageData.frontmatter.title} · Dokyr` : 'Dokyr';
    const image = `${siteOrigin}/og-image.png`;
    const routeParts = pageData.relativePath
      .replace(/\.md$/, '')
      .split('/')
      .filter((part) => part && part !== 'index');
    const breadcrumbParts = routeParts.map((part, index) => ({
      '@type': 'ListItem',
      position: index + 2,
      name:
        index === routeParts.length - 1 && pageData.frontmatter.title
          ? pageData.frontmatter.title
          : part.replace(/-/g, ' ').replace(/\b\w/g, (letter) => letter.toUpperCase()),
      item: `${siteOrigin}/${routeParts.slice(0, index + 1).join('/')}`
    }));
    const pageSchema = {
      '@context': 'https://schema.org',
      '@graph': [
        {
          '@type': 'WebPage',
          name: pageTitle,
          description: pageDescription,
          url,
          inLanguage: 'en-US',
          isPartOf: {
            '@type': 'WebSite',
            name: 'Dokyr',
            url: `${siteOrigin}/`
          }
        },
        ...(routeParts.length
          ? [
              {
                '@type': 'BreadcrumbList',
                itemListElement: [
                  {
                    '@type': 'ListItem',
                    position: 1,
                    name: 'Dokyr',
                    item: `${siteOrigin}/`
                  },
                  ...breadcrumbParts
                ]
              }
            ]
          : [])
      ]
    };

    return [
      ['link', { rel: 'canonical', href: url }],
      ['meta', { property: 'og:title', content: pageTitle }],
      ['meta', { property: 'og:description', content: pageDescription }],
      ['meta', { property: 'og:url', content: url }],
      ['meta', { property: 'og:image', content: image }],
      ['meta', { property: 'og:image:width', content: '1200' }],
      ['meta', { property: 'og:image:height', content: '630' }],
      ['meta', { property: 'og:image:alt', content: 'Dokyr — your server, your platform' }],
      ['meta', { name: 'twitter:title', content: pageTitle }],
      ['meta', { name: 'twitter:description', content: pageDescription }],
      ['meta', { name: 'twitter:image', content: image }],
      ['meta', { name: 'twitter:image:alt', content: 'Dokyr — your server, your platform' }],
      ['script', { type: 'application/ld+json' }, JSON.stringify(pageSchema)]
    ];
  },
  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'Dokyr',
    nav: [
      { text: 'Guide', link: '/guide/' },
      { text: 'Infrastructure', link: '/infrastructure/domains' },
      { text: 'Operations', link: '/operations/upgrades' },
      { text: 'API', link: '/API' },
      { text: 'Architecture', link: '/ARCHITECTURE' },
      { text: 'v0.2.32', link: 'https://github.com/azayr/dokyr/releases/tag/v0.2.32' }
    ],
    sidebar: {
      '/guide/': [
        {
          text: 'Get started',
          items: [
            { text: 'What is Dokyr?', link: '/guide/' },
            { text: 'Install on a VPS', link: '/guide/installation' },
            { text: 'Create your first project', link: '/guide/first-project' },
            { text: 'Deployments', link: '/guide/deployments' }
          ]
        }
      ],
      '/infrastructure/': [
        {
          text: 'Infrastructure',
          items: [
            { text: 'Domains and HTTPS', link: '/infrastructure/domains' },
            { text: 'Private registry', link: '/infrastructure/registry' },
            { text: 'Object storage', link: '/infrastructure/storage' },
            { text: 'Developer mail', link: '/infrastructure/mail' }
          ]
        }
      ],
      '/operations/': [
        {
          text: 'Operations',
          items: [
            { text: 'Updates', link: '/operations/upgrades' },
            { text: 'Backups and restores', link: '/operations/backups' },
            { text: 'Security model', link: '/operations/security' }
          ]
        }
      ],
      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'Configuration', link: '/reference/configuration' },
            { text: 'API reference', link: '/API' },
            { text: 'Architecture', link: '/ARCHITECTURE' }
          ]
        }
      ]
    },
    search: {
      provider: 'local'
    },
    outline: {
      level: [2, 3],
      label: 'On this page'
    },
    socialLinks: [{ icon: 'github', link: repository }],
    editLink: {
      pattern: `${repository}/edit/main/docs/:path`,
      text: 'Edit this page on GitHub'
    },
    footer: {
      message: 'Open source infrastructure, operated on your terms.',
      copyright: 'Built in public by Dokyr contributors.'
    }
  }
}));
