<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { withBase } from 'vitepress';

const installCommand = 'curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/scripts/install.sh | sudo sh';
const copied = ref(false);

const features = [
  {
    code: '01 / SHIP',
    title: 'Deploy containers without the platform tax.',
    text: 'Pull a public or private image or import image services from Compose. Dokyr records every stage and keeps the previous release ready for rollback.',
    className: 'feature-wide',
    visual: 'pipeline'
  },
  {
    code: '02 / ROUTE',
    title: 'Domains become configuration, not ceremony.',
    text: 'Route hostnames and paths to individual services while Caddy handles certificates and atomic configuration updates.',
    className: 'feature-tall',
    visual: 'routes'
  },
  {
    code: '03 / STORE',
    title: 'Private data services, close to the app.',
    text: 'Provision PostgreSQL, MySQL, or MariaDB with persistent volumes. Publish a port only when you deliberately choose to.',
    className: 'feature-standard',
    visual: 'database'
  },
  {
    code: '04 / DISTRIBUTE',
    title: 'Your own registry, with scoped credentials.',
    text: 'Push and pull through the built-in Docker Distribution registry, backed by local storage or an S3-compatible service.',
    className: 'feature-standard',
    visual: 'registry'
  },
  {
    code: '05 / DELIVER',
    title: 'Verified developer mail is built in.',
    text: 'Connect a sending domain, verify DNS, and issue domain-scoped credentials for SMTP or a small Resend-style HTTP API.',
    className: 'feature-wide',
    visual: 'mail'
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
    <div class="page-rail page-rail-left" aria-hidden="true"></div>
    <div class="page-rail page-rail-right" aria-hidden="true"></div>

    <section class="hero section-frame" aria-labelledby="hero-title">
      <div class="corner corner-tl" aria-hidden="true"></div>
      <div class="corner corner-tr" aria-hidden="true"></div>
      <div class="hero-copy" data-reveal>
        <a class="release-link" href="https://github.com/azayr/dokyr/releases/tag/v0.2.32">
          <span>v0.2.32</span>
          latest release
          <b aria-hidden="true">↗</b>
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
            Install Dokyr <span aria-hidden="true">→</span>
          </a>
          <a class="button button-secondary" href="https://github.com/azayr/dokyr">
            View source <span aria-hidden="true">↗</span>
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
          <p><span class="console-time">09:42:11</span><i class="ok">✓</i><span class="console-message">prepare release</span></p>
          <p><span class="console-time">09:42:12</span><i class="ok">✓</i><span class="console-message">pull immutable image</span></p>
          <p><span class="console-time">09:42:18</span><i class="ok">✓</i><span class="console-message">create replacement container</span></p>
          <p><span class="console-time">09:42:20</span><i class="ok">✓</i><span class="console-message">verify <b>GET /health</b></span></p>
          <p class="active"><span class="console-time">09:42:21</span><i>→</i><span class="console-message">promote release behind Caddy</span></p>
        </div>
        <div class="console-result">
          <div><span>STATUS</span><strong>HEALTHY</strong></div>
          <div><span>DURATION</span><strong>10.4s</strong></div>
          <div><span>HTTPS</span><strong>ACTIVE</strong></div>
        </div>
      </div>

      <div class="install-bar" data-reveal>
        <span class="prompt" aria-hidden="true">$</span>
        <code>{{ installCommand }}</code>
        <button type="button" @click="copyInstall">{{ copied ? 'COPIED' : 'COPY' }}</button>
      </div>
    </section>

    <div class="stack-strip section-frame" aria-label="Dokyr technology stack" data-reveal>
      <span class="stack-label">BUILT FROM BORING, PROVEN PARTS</span>
      <div class="stack-list">
        <span><b>GO</b> control plane</span>
        <span><b>SVELTE</b> interface</span>
        <span><b>DOCKER</b> runtime</span>
        <span><b>CADDY</b> edge</span>
        <span><b>POSTGRESQL</b> state</span>
        <span><b>STALWART</b> mail</span>
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
        <article>
          <span>CONTROL</span>
          <strong>Your machine stays yours.</strong>
          <p>Workloads, secrets, certificates, databases, and volumes remain on infrastructure you operate.</p>
        </article>
        <article>
          <span>CLARITY</span>
          <strong>Every deployment has a story.</strong>
          <p>Follow pull, create, verify, promote, and rollback events without decoding a black box.</p>
        </article>
        <article>
          <span>FOCUS</span>
          <strong>One host, done carefully.</strong>
          <p>Designed for indie products, internal tools, agencies, and small teams that do not need a cluster.</p>
        </article>
      </div>
    </section>

    <section class="capabilities section-frame" aria-labelledby="capabilities-title">
      <div class="section-heading compact" data-reveal>
        <p class="eyebrow">// one control plane, the whole shipping loop</p>
        <h2 id="capabilities-title">From image to <em>internet.</em></h2>
      </div>

      <div class="feature-grid">
        <article v-for="feature in features" :key="feature.code" :class="['feature-card', feature.className]" data-reveal>
          <div class="feature-copy">
            <span class="feature-code">{{ feature.code }}</span>
            <h3>{{ feature.title }}</h3>
            <p>{{ feature.text }}</p>
          </div>

          <div v-if="feature.visual === 'pipeline'" class="feature-visual pipeline-visual" aria-hidden="true">
            <div class="pipeline-head"><span>RELEASE / api@2.4.1</span><b>10.4s</b></div>
            <div class="pipeline-row"><i class="done"></i><span>pull</span><small>6.2s</small></div>
            <div class="pipeline-row"><i class="done"></i><span>create</span><small>1.1s</small></div>
            <div class="pipeline-row"><i class="done"></i><span>verify</span><small>2.4s</small></div>
            <div class="pipeline-row"><i class="live"></i><span>promote</span><small>0.7s</small></div>
          </div>

          <div v-else-if="feature.visual === 'routes'" class="feature-visual route-visual" aria-hidden="true">
            <div class="domain-chip">api.example.com</div>
            <span class="route-line"></span>
            <div class="route-target"><small>/api/*</small><b>API :8080</b></div>
            <div class="route-target"><small>/*</small><b>WEB :3000</b></div>
            <div class="certificate"><i></i> TLS certificate active</div>
          </div>

          <div v-else-if="feature.visual === 'database'" class="feature-visual database-visual" aria-hidden="true">
            <div class="db-cylinder"><i></i><i></i><i></i></div>
            <div><span>POSTGRES 17</span><strong>healthy</strong><small>private network · persistent volume</small></div>
          </div>

          <div v-else-if="feature.visual === 'registry'" class="feature-visual registry-visual" aria-hidden="true">
            <div class="package-row"><i></i><span>acme/web</span><b>2.8.0</b></div>
            <div class="package-row"><i></i><span>acme/api</span><b>2.4.1</b></div>
            <div class="package-row"><i></i><span>acme/worker</span><b>1.9.3</b></div>
          </div>

          <div v-else class="feature-visual mail-visual" aria-hidden="true">
            <div class="mail-header"><span>POST /v1/emails</span><strong>202 QUEUED</strong></div>
            <code>{<br />&nbsp;&nbsp;"from": "hello@updates.example.com",<br />&nbsp;&nbsp;"to": ["team@example.net"]<br />}</code>
            <div class="dns-row"><i></i> SPF</div><div class="dns-row"><i></i> DKIM</div><div class="dns-row"><i></i> DMARC</div>
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
        <div class="arch-internet"><span>INTERNET</span></div>
        <div class="arch-line line-edge"></div>
        <div class="arch-node node-caddy"><span>EDGE</span><strong>Caddy</strong><small>HTTP · HTTPS · certificates</small></div>
        <div class="arch-line line-control"></div>
        <div class="arch-node node-dokyr"><span>CONTROL</span><strong>Dokyr</strong><small>Go API · Svelte UI</small></div>
        <div class="arch-line line-services"></div>
        <div class="arch-services">
          <div><span>STATE</span><strong>PostgreSQL</strong></div>
          <div><span>IMAGES</span><strong>Registry</strong></div>
          <div><span>MAIL</span><strong>Stalwart</strong></div>
        </div>
        <div class="arch-socket"><span>UNIX SOCKET</span><strong>Docker Engine</strong></div>
        <div class="arch-workloads"><i></i><i></i><i></i><span>YOUR WORKLOADS</span></div>
      </div>

      <a class="text-link" :href="withBase('/ARCHITECTURE')" data-reveal>
        Read the complete architecture <span aria-hidden="true">→</span>
      </a>
    </section>

    <section class="open-source section-frame" aria-labelledby="open-source-title">
      <div class="open-source-mark" aria-hidden="true">OPEN<br />SOURCE</div>
      <div class="open-source-copy" data-reveal>
        <p class="eyebrow">// fork it, inspect it, make it yours</p>
        <h2 id="open-source-title">No platform should require <em>blind trust.</em></h2>
        <p>
          Dokyr is built in public. Read the Go control plane, inspect every migration, follow the Docker API calls,
          and contribute improvements that make the platform work better for small teams.
        </p>
        <div class="open-source-links">
          <a href="https://github.com/azayr/dokyr">GitHub repository <span aria-hidden="true">↗</span></a>
          <a :href="withBase('/guide/')">Read the docs <span aria-hidden="true">→</span></a>
          <a :href="withBase('/API')">Explore the API <span aria-hidden="true">→</span></a>
        </div>
      </div>
    </section>

    <section class="final-cta section-frame" aria-labelledby="cta-title">
      <div class="cta-grid" aria-hidden="true"></div>
      <div data-reveal>
        <p class="eyebrow">// a VPS is enough</p>
        <h2 id="cta-title">Make one server feel like <em>your cloud.</em></h2>
        <p>Install Dokyr, create the owner account, and ship your first container.</p>
        <div class="hero-actions centered">
          <a class="button button-primary" :href="withBase('/guide/installation')">Start the installation <span aria-hidden="true">→</span></a>
          <a class="button button-secondary" href="https://github.com/azayr/dokyr">Read the source <span aria-hidden="true">↗</span></a>
        </div>
      </div>
    </section>
  </main>
</template>
