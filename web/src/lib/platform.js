import { writable } from 'svelte/store';
import { api } from './auth.js';

export const platformUpdate = writable(null);

export function formatPlatformVersion(value, fallback = 'Unavailable') {
  const version = String(value || '').trim();
  if (!version) return fallback;
  if (/^(sha-)?[0-9a-f]{7,}$/i.test(version)) return 'Unversioned build';
  if (/^\d/.test(version)) return `v${version}`;
  if (version === 'dev') return 'Development';
  return version;
}

export async function loadPlatformUpdate(refresh = false) {
  const response = await api(`/api/settings/platform/update${refresh ? '?refresh=true' : ''}`);
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || 'Could not load platform version.');
  platformUpdate.set(data);
  return data;
}

export async function checkPlatformUpdate() {
  const response = await api('/api/settings/platform/update/check', { method: 'POST' });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || 'Could not check for updates.');
  platformUpdate.set(data);
  return data;
}
