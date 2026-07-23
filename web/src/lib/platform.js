import { writable } from 'svelte/store';
import { api } from './auth.js';

export const platformUpdate = writable(null);

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
