<script>
  export let name = 'box';
  export let size = 18;
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
{#if name==='grid'}<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>
{:else if name==='server'}<rect x="3" y="4" width="18" height="6" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/><path d="M7 7h.01M7 17h.01"/>
{:else if name==='database'}<ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v7c0 1.7 3.6 3 8 3s8-1.3 8-3V5"/><path d="M4 12v7c0 1.7 3.6 3 8 3s8-1.3 8-3v-7"/>
{:else if name==='rocket'}<path d="M4.5 16.5c-1.5 1.3-2 5-2 5s3.7-.5 5-2c.7-.8.7-2 0-2.7-.7-.7-2-.7-3 .2z"/><path d="m9 15-3-3s3.5-6.5 10-9c2.3-.9 5 0 5 0s.9 2.7 0 5c-2.5 6.5-9 10-9 10l-3-3z"/><circle cx="16" cy="8" r="1.5"/>
{:else if name==='activity'}<path d="M3 12h4l2-7 4 14 2-7h6"/>
{:else if name==='settings'}<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H2.8v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1A1.7 1.7 0 0 0 9 4.6 1.7 1.7 0 0 0 10 3V2.8h4V3a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1z"/>
{:else if name==='git'}<circle cx="6" cy="3" r="2"/><circle cx="18" cy="6" r="2"/><circle cx="6" cy="21" r="2"/><path d="M6 5v14M8 7c8 0 8-1 8-1"/>
{:else if name==='globe'}<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c2.5 2.5 4 5.5 4 9s-1.5 6.5-4 9c-2.5-2.5-4-5.5-4-9s1.5-6.5 4-9z"/>
{:else if name==='mail'}<rect x="3" y="5" width="18" height="14" rx="2"/><path d="m4 7 8 6 8-6"/>
{:else if name==='search'}<circle cx="11" cy="11" r="7"/><path d="m16.5 16.5 4 4"/>
{:else if name==='moon'}<path d="M20.5 14.2A8 8 0 0 1 9.8 3.5 8.5 8.5 0 1 0 20.5 14.2z"/>
{:else if name==='sun'}<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.42 1.42M17.65 17.65l1.42 1.42M2 12h2M20 12h2M4.93 19.07l1.42-1.42M17.65 6.35l1.42-1.42"/>
{:else if name==='monitor'}<rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/>
{:else if name==='plus'}<path d="M12 5v14M5 12h14"/>
{:else if name==='logs'}<path d="M5 5h14M5 10h14M5 15h9M5 20h6"/>
{:else if name==='play'}<path d="m8 5 11 7-11 7V5z"/>
{:else if name==='stop'}<rect x="6" y="6" width="12" height="12" rx="1.5"/>
{:else if name==='trash'}<path d="M4 7h16M9 7V4h6v3M7 7l1 13h8l1-13M10 11v5M14 11v5"/>
{:else if name==='key'}<circle cx="8" cy="15" r="4"/><path d="m11 12 8-8M16 7l2 2M14 9l2 2"/>
{:else if name==='network'}<circle cx="5" cy="12" r="2"/><circle cx="19" cy="6" r="2"/><circle cx="19" cy="18" r="2"/><path d="m7 11 10-4M7 13l10 4"/>
{:else if name==='refresh'}<path d="M20 7v5h-5M4 17v-5h5"/><path d="M6.1 8.5A7 7 0 0 1 18 7l2 5M4 12l2 5a7 7 0 0 0 11.9-1.5"/>
{:else if name==='shield'}<path d="M12 3 20 6v5c0 5-3.4 8.4-8 10-4.6-1.6-8-5-8-10V6l8-3z"/><path d="m9 12 2 2 4-5"/>
{:else if name==='lock'}<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3M12 14v3"/>
{:else if name==='link'}<path d="M10 13a5 5 0 0 0 7.5.5l2-2a5 5 0 0 0-7-7l-1.1 1.1"/><path d="M14 11a5 5 0 0 0-7.5-.5l-2 2a5 5 0 0 0 7 7l1.1-1.1"/>
{:else if name==='copy'}<rect x="8" y="8" width="11" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h2"/>
{:else if name==='check'}<path d="m5 12 4 4L19 6"/>
{:else if name==='check-circle'}<circle cx="12" cy="12" r="9"/><path d="m8.5 12.5 2.5 2.5 5-6"/>
{:else if name==='x'}<path d="M6 6l12 12M18 6 6 18"/>
{:else if name==='x-circle'}<circle cx="12" cy="12" r="9"/><path d="m9 9 6 6M15 9l-6 6"/>
{:else if name==='chevron-down'}<path d="m6 9 6 6 6-6"/>
{:else if name==='chevron-right'}<path d="m9 6 6 6-6 6"/>
{:else if name==='chevron-left'}<path d="m15 6-6 6 6 6"/>
{:else if name==='menu'}<path d="M4 6h16M4 12h16M4 18h16"/>
{:else if name==='bell'}<path d="M6 9a6 6 0 0 1 12 0c0 5 2 6 2 6H4s2-1 2-6"/><path d="M10 19a2 2 0 0 0 4 0"/>
{:else if name==='logout'}<path d="M9 21H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3"/><path d="m16 17 5-5-5-5M21 12H9"/>
{:else if name==='external'}<path d="M14 4h6v6"/><path d="M20 4 10 14"/><path d="M20 14v5a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h5"/>
{:else if name==='eye'}<path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12z"/><circle cx="12" cy="12" r="3"/>
{:else if name==='eye-off'}<path d="M4 4l16 16"/><path d="M10.6 6.1A9.8 9.8 0 0 1 12 6c6 0 9.5 6 9.5 6a17 17 0 0 1-2.9 3.7M6.6 6.9A16.7 16.7 0 0 0 2.5 12S6 18 12 18a9.6 9.6 0 0 0 3.9-.8"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2"/>
{:else if name==='command'}<path d="M9 9V6a3 3 0 1 0-3 3h3zm0 0v6m0-6h6m-6 6v3a3 3 0 1 1-3-3h3zm6-6h3a3 3 0 1 0-3-3v3zm0 0v6m0 0h3a3 3 0 1 1-3 3v-3z"/>
{:else if name==='layers'}<path d="m12 3 9 5-9 5-9-5 9-5z"/><path d="m3 13 9 5 9-5"/>
{:else if name==='cpu'}<rect x="6" y="6" width="12" height="12" rx="2"/><rect x="10" y="10" width="4" height="4"/><path d="M9 2v2M15 2v2M9 20v2M15 20v2M2 9h2M2 15h2M20 9h2M20 15h2"/>
{:else if name==='hard-drive'}<path d="M4 14 6.5 6h11L20 14"/><path d="M4 14h16v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-4z"/><path d="M16.5 17h.01"/>
{:else if name==='clock'}<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>
{:else if name==='filter'}<path d="M4 5h16l-6.5 8v6l-3-2v-4L4 5z"/>
{:else if name==='arrow-right'}<path d="M5 12h14m-6-6 6 6-6 6"/>
{:else if name==='arrow-left'}<path d="M19 12H5m6 6-6-6 6-6"/>
{:else if name==='alert'}<path d="M12 4 2.5 20h19L12 4z"/><path d="M12 10v4M12 17.5h.01"/>
{:else if name==='info'}<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 7.5h.01"/>
{:else if name==='folder'}<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"/>
{:else if name==='terminal'}<path d="m5 8 4 4-4 4M12 17h7"/><rect x="2.5" y="4" width="19" height="16" rx="2"/>
{:else if name==='user'}<circle cx="12" cy="8" r="4"/><path d="M4.5 20a7.5 7.5 0 0 1 15 0"/>
{:else if name==='more'}<circle cx="5" cy="12" r="1" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r="1" fill="currentColor" stroke="none"/><circle cx="19" cy="12" r="1" fill="currentColor" stroke="none"/>
{:else if name==='home'}<path d="m4 11 8-7 8 7"/><path d="M6 10v9a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1v-9"/>
{:else if name==='file-text'}<path d="M13 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V9l-6-6z"/><path d="M13 3v6h6M9 13h6M9 17h6"/>
{:else if name==='zap'}<path d="M13 2 4.5 13.5H11L10 22l8.5-11.5H13L13 2z"/>
{:else if name==='github'}<path fill="currentColor" stroke="none" d="M12 2C6.48 2 2 6.59 2 12.26c0 4.54 2.87 8.39 6.84 9.75.5.1.68-.22.68-.49v-1.93c-2.78.62-3.37-1.21-3.37-1.21-.45-1.19-1.11-1.5-1.11-1.5-.91-.64.07-.63.07-.63 1 .07 1.53 1.06 1.53 1.06.89 1.57 2.34 1.12 2.91.86.09-.66.35-1.12.63-1.38-2.22-.26-4.56-1.14-4.56-5.07 0-1.12.39-2.03 1.03-2.75-.1-.26-.45-1.3.1-2.71 0 0 .84-.28 2.75 1.05A9.3 9.3 0 0 1 12 6.97a9.3 9.3 0 0 1 2.5.35c1.91-1.33 2.75-1.05 2.75-1.05.55 1.41.2 2.45.1 2.71.64.72 1.03 1.63 1.03 2.75 0 3.94-2.34 4.8-4.57 5.06.36.32.68.94.68 1.9v2.82c0 .27.18.59.69.49A10.27 10.27 0 0 0 22 12.26C22 6.59 17.52 2 12 2z"/>
{:else if name==='gitlab'}<path fill="currentColor" stroke="none" d="M12 21.4 16.6 9.3H7.4L12 21.4zM12 21.4 7.4 9.3H1.9L12 21.4zM1.9 9.3l1.4-4.3c.1-.4.5-.4.7 0l1.9 4.3H1.9zM12 21.4l4.6-12.1h5.5L12 21.4zM22.1 9.3l-1.4-4.3c-.1-.4-.5-.4-.7 0l-1.9 4.3h4z"/>
{:else}<path d="m21 8-9 5-9-5 9-5 9 5z"/><path d="m3 8 9 5 9-5v8l-9 5-9-5V8z"/>{/if}
</svg>
