<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { withBase } from 'vitepress';
import { Icon } from '@iconify/vue';
import arrowRightIcon from '@iconify-icons/lucide/arrow-right';
import badgeCheckIcon from '@iconify-icons/lucide/badge-check';
import bookOpenIcon from '@iconify-icons/lucide/book-open';
import boxesIcon from '@iconify-icons/lucide/boxes';
import boxIcon from '@iconify-icons/lucide/box';
import checkIcon from '@iconify-icons/lucide/check';
import cloudIcon from '@iconify-icons/lucide/cloud';
import codeIcon from '@iconify-icons/lucide/code-2';
import containerIcon from '@iconify-icons/lucide/container';
import copyIcon from '@iconify-icons/lucide/copy';
import cpuIcon from '@iconify-icons/lucide/cpu';
import databaseIcon from '@iconify-icons/lucide/database';
import externalLinkIcon from '@iconify-icons/lucide/external-link';
import focusIcon from '@iconify-icons/lucide/focus';
import globeIcon from '@iconify-icons/lucide/globe-2';
import githubIcon from '@iconify-icons/lucide/github';
import layersIcon from '@iconify-icons/lucide/layers';
import lockIcon from '@iconify-icons/lucide/lock-keyhole';
import mailIcon from '@iconify-icons/lucide/mail';
import monitorIcon from '@iconify-icons/lucide/monitor';
import packageIcon from '@iconify-icons/lucide/package';
import rocketIcon from '@iconify-icons/lucide/rocket';
import routeIcon from '@iconify-icons/lucide/git-branch';
import scaleIcon from '@iconify-icons/lucide/scale';
import serverIcon from '@iconify-icons/lucide/server';
import terminalIcon from '@iconify-icons/lucide/terminal';
import { releaseTag, releaseURL } from '../release';

const installCommand = 'curl -fsSL https://sh.dokyr.com | sudo sh';
const copied = ref(false);

const stack = [
  { name: 'GO', label: 'control plane', icon: cpuIcon },
  { name: 'SVELTE', label: 'interface', icon: monitorIcon },
  { name: 'DOCKER', label: 'runtime', icon: containerIcon },
  { name: 'CADDY', label: 'edge', icon: cloudIcon },
  { name: 'POSTGRESQL', label: 'state', icon: databaseIcon },
  { name: 'STALWART', label: 'mail', icon: mailIcon }
];

const principles = [
  {
    label: 'CONTROL',
    title: 'Your machine stays yours.',
    text: 'Workloads, secrets, certificates, databases, and volumes remain on infrastructure you operate.',
    icon: lockIcon
  },
  {
    label: 'CLARITY',
    title: 'Every deployment has a story.',
    text: 'Follow pull, create, verify, promote, and rollback events without decoding a black box.',
    icon: layersIcon
  },
  {
    label: 'FOCUS',
    title: 'One host, done carefully.',
    text: 'Designed for indie products, internal tools, agencies, and small teams that do not need a cluster.',
    icon: focusIcon
  }
];

const features = [
  {
    code: '01 / SHIP',
    title: 'Deploy containers without the platform tax.',
    text: 'Pull a public or private image or import image services from Compose. Dokyr records every stage and keeps the previous release ready for rollback.',
    icon: rocketIcon
  },
  {
    code: '02 / ROUTE',
    title: 'Domains become configuration, not ceremony.',
    text: 'Route hostnames and paths to individual services while Caddy handles certificates and atomic configuration updates.',
    icon: routeIcon
  },
  {
    code: '03 / STORE',
    title: 'Private data services, close to the app.',
    text: 'Provision PostgreSQL, MySQL, or MariaDB with persistent volumes. Publish a port only when you deliberately choose to.',
    icon: databaseIcon
  },
  {
    code: '04 / DISTRIBUTE',
    title: 'Your own registry, with scoped credentials.',
    text: 'Push and pull through the built-in Docker Distribution registry, backed by local storage or an S3-compatible service.',
    icon: packageIcon
  },
  {
    code: '05 / DELIVER',
    title: 'Verified developer mail is built in.',
    text: 'Connect a sending domain, verify DNS, and issue domain-scoped credentials for SMTP or a small Resend-style HTTP API.',
    icon: mailIcon
  }
];

async function copyInstall() {
  await navigator.clipboard.writeText(installCommand);
  copied.value = true;
  window.setTimeout(() => (copied.value = false), 1800);
}

onMounted(() => {
  const items = document.querySelectorAll<HTMLElement>('[data-reveal]');
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    items.forEach((item) => item.classList.add('is-visible'));
    return;
  }

  const observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        entry.target.classList.add('is-visible');
        observer.unobserve(entry.target);
      }
    },
    { threshold: 0.12 }
  );

  items.forEach((item) => observer.observe(item));
});
</script>

<template>
  <main class="landing-shell">
    <section class="hero section-frame" aria-labelledby="hero-title">
      <div class="hero-copy" data-reveal>
        <a class="release-link" :href="releaseURL">
          <Icon :icon="badgeCheckIcon" aria-hidden="true" />
          <span>{{ releaseTag }}</span>
          latest release
          <Icon class="link-icon" :icon="externalLinkIcon" aria-hidden="true" />
        </a>
        <p class="eyebrow">Open-source · single-server PaaS</p>
        <h1 id="hero-title">
          Own your server.<br />
          <em>Ship like a platform.</em>
        </h1>
        <p class="hero-lede">
          Dokyr turns one Docker host into a calm deployment control plane—with projects, releases, domains,
          databases, registry, storage, and mail under your control.
        </p>
        <div class="hero-actions">
          <a class="button button-primary" :href="withBase('/guide/installation')">
            <Icon :icon="terminalIcon" aria-hidden="true" /> Install Dokyr
          </a>
          <a class="button button-secondary" href="https://github.com/azayr/dokyr">
            <Icon :icon="githubIcon" aria-hidden="true" /> View source
          </a>
        </div>
      </div>

      <div class="hero-console" data-reveal aria-label="Example Dokyr deployment output">
        <div class="console-glow" aria-hidden="true"></div>
        <div class="console-topbar">
          <div class="console-dots" aria-hidden="true"><i></i><i></i><i></i></div>
          <span>deployment / production-api</span>
          <b>live</b>
        </div>
        <div class="console-meta">
          <span>DEPLOYMENT</span><strong>dep_6F2A9</strong>
          <span>SOURCE</span><strong>ghcr.io/acme/api:2.4.1</strong>
          <span>TARGET</span><strong>api.example.com</strong>
        </div>
        <div class="console-log" role="presentation">
          <p><span class="console-time">09:42:11</span><i class="ok"><Icon :icon="checkIcon" /></i><span class="console-message">prepare release</span></p>
          <p><span class="console-time">09:42:12</span><i class="ok"><Icon :icon="checkIcon" /></i><span class="console-message">pull immutable image</span></p>
          <p><span class="console-time">09:42:18</span><i class="ok"><Icon :icon="checkIcon" /></i><span class="console-message">create replacement container</span></p>
          <p><span class="console-time">09:42:20</span><i class="ok"><Icon :icon="checkIcon" /></i><span class="console-message">verify <b>GET /health</b></span></p>
          <p class="active"><span class="console-time">09:42:21</span><i><Icon :icon="arrowRightIcon" /></i><span class="console-message">promote release behind Caddy</span></p>
        </div>
        <div class="console-result">
          <div><span>STATUS</span><strong>HEALTHY</strong></div>
          <div><span>DURATION</span><strong>10.4s</strong></div>
          <div><span>HTTPS</span><strong>ACTIVE</strong></div>
        </div>
      </div>

      <div class="install-bar" data-reveal>
        <Icon class="prompt" :icon="terminalIcon" aria-hidden="true" />
        <code>{{ installCommand }}</code>
        <button type="button" @click="copyInstall">
          <Icon :icon="copied ? checkIcon : copyIcon" aria-hidden="true" /> {{ copied ? 'COPIED' : 'COPY' }}
        </button>
      </div>
    </section>

    <div class="stack-strip section-frame" aria-label="Dokyr technology stack" data-reveal>
      <span class="stack-label">BUILT FROM BORING, PROVEN PARTS</span>
      <div class="stack-list">
        <span v-for="item in stack" :key="item.name">
          <Icon :icon="item.icon" aria-hidden="true" /><b>{{ item.name }}</b><small>{{ item.label }}</small>
        </span>
      </div>
    </div>

    <section class="positioning section-frame" aria-labelledby="positioning-title">
      <div class="section-heading" data-reveal>
        <p class="eyebrow">// infrastructure without abstraction debt</p>
        <h2 id="positioning-title">The useful part of a cloud platform.<br /><em>On a server you can explain.</em></h2>
        <p>
          Dokyr is intentionally single-node. There is no hidden scheduler, proprietary build plane, or per-seat
          invoice—just an auditable control plane over Docker and Caddy.
        </p>
      </div>
      <div class="principles" data-reveal>
        <article v-for="principle in principles" :key="principle.label">
          <Icon class="principle-icon" :icon="principle.icon" aria-hidden="true" />
          <span>{{ principle.label }}</span>
          <strong>{{ principle.title }}</strong>
          <p>{{ principle.text }}</p>
        </article>
      </div>
    </section>

    <section class="capabilities section-frame" aria-labelledby="capabilities-title">
      <div class="section-heading compact" data-reveal>
        <p class="eyebrow">// one control plane, the whole shipping loop</p>
        <h2 id="capabilities-title">From image to <em>internet.</em></h2>
      </div>

      <div class="feature-grid">
        <article v-for="feature in features" :key="feature.code" class="feature-card" data-reveal>
          <div class="feature-copy">
            <div class="feature-kicker">
              <span class="feature-icon" aria-hidden="true"><Icon :icon="feature.icon" /></span>
              <span class="feature-code">{{ feature.code }}</span>
            </div>
            <h3>{{ feature.title }}</h3>
            <p>{{ feature.text }}</p>
          </div>
        </article>
      </div>
    </section>

    <section class="architecture section-frame" aria-labelledby="architecture-title">
      <div class="section-heading" data-reveal>
        <p class="eyebrow">// visible systems are operable systems</p>
        <h2 id="architecture-title">Small enough to <em>understand.</em></h2>
        <p>Five platform containers, two intentional networks, and one protected Docker socket.</p>
      </div>

      <div class="architecture-board" data-reveal>
        <div class="arch-internet"><Icon class="arch-icon" :icon="globeIcon" aria-hidden="true" /><span>INTERNET</span></div>
        <div class="arch-line line-edge"></div>
        <div class="arch-node node-caddy"><Icon class="arch-icon" :icon="cloudIcon" aria-hidden="true" /><span>EDGE</span><strong>Caddy</strong><small>HTTP · HTTPS · certificates</small></div>
        <div class="arch-line line-control"></div>
        <div class="arch-node node-dokyr"><Icon class="arch-icon" :icon="boxesIcon" aria-hidden="true" /><span>CONTROL</span><strong>Dokyr</strong><small>Go API · Svelte UI</small></div>
        <div class="arch-line line-services"></div>
        <div class="arch-services">
          <div><Icon class="arch-icon" :icon="databaseIcon" aria-hidden="true" /><span>STATE</span><strong>PostgreSQL</strong></div>
          <div><Icon class="arch-icon" :icon="packageIcon" aria-hidden="true" /><span>IMAGES</span><strong>Registry</strong></div>
          <div><Icon class="arch-icon" :icon="mailIcon" aria-hidden="true" /><span>MAIL</span><strong>Stalwart</strong></div>
        </div>
        <div class="arch-socket"><Icon class="arch-icon" :icon="containerIcon" aria-hidden="true" /><span>UNIX SOCKET</span><strong>Docker Engine</strong></div>
        <div class="arch-workloads"><i><Icon :icon="boxIcon" /></i><i><Icon :icon="boxIcon" /></i><i><Icon :icon="boxIcon" /></i><span>YOUR WORKLOADS</span></div>
      </div>

      <a class="text-link" :href="withBase('/ARCHITECTURE')" data-reveal>
        <Icon :icon="bookOpenIcon" aria-hidden="true" /> Read the complete architecture <Icon :icon="arrowRightIcon" aria-hidden="true" />
      </a>
    </section>

    <section class="open-source section-frame" aria-labelledby="open-source-title">
      <div class="open-source-mark" aria-hidden="true"><Icon :icon="githubIcon" /></div>
      <div class="open-source-copy" data-reveal>
        <p class="eyebrow">// fork it, inspect it, make it yours</p>
        <h2 id="open-source-title">No platform should require <em>blind trust.</em></h2>
        <p>
          Dokyr is built in public. Read the Go control plane, inspect every migration, follow the Docker API calls,
          and contribute improvements that make the platform work better for small teams. Use, modify, and distribute
          it under the permissive MIT License.
        </p>
        <div class="open-source-links">
          <a href="https://github.com/azayr/dokyr"><Icon :icon="githubIcon" aria-hidden="true" />GitHub repository <Icon :icon="externalLinkIcon" aria-hidden="true" /></a>
          <a href="https://github.com/azayr/dokyr/blob/main/LICENSE"><Icon :icon="scaleIcon" aria-hidden="true" />MIT License <Icon :icon="externalLinkIcon" aria-hidden="true" /></a>
          <a :href="withBase('/guide/')"><Icon :icon="bookOpenIcon" aria-hidden="true" />Read the docs <Icon :icon="arrowRightIcon" aria-hidden="true" /></a>
          <a :href="withBase('/API')"><Icon :icon="codeIcon" aria-hidden="true" />Explore the API <Icon :icon="arrowRightIcon" aria-hidden="true" /></a>
        </div>
      </div>
    </section>

    <section class="final-cta section-frame" aria-labelledby="cta-title">
      <div class="cta-grid" aria-hidden="true"></div>
      <div data-reveal>
        <span class="cta-icon" aria-hidden="true"><Icon :icon="serverIcon" /></span>
        <p class="eyebrow">// a VPS is enough</p>
        <h2 id="cta-title">Make one server feel like <em>your cloud.</em></h2>
        <p>Install Dokyr, create the owner account, and ship your first container.</p>
        <div class="hero-actions centered">
          <a class="button button-primary" :href="withBase('/guide/installation')"><Icon :icon="terminalIcon" aria-hidden="true" />Start the installation</a>
          <a class="button button-secondary" href="https://github.com/azayr/dokyr"><Icon :icon="githubIcon" aria-hidden="true" />Read the source</a>
        </div>
      </div>
    </section>
  </main>
</template>
