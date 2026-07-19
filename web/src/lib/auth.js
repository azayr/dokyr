import { writable } from 'svelte/store';

export const currentUser = writable(null);

export async function loadSession() {
  const response = await fetch('/api/auth/me');
  if (!response.ok) { currentUser.set(null); return null; }
  const data = await response.json();
  currentUser.set(data.user);
  return data.user;
}

export async function logout() {
  await fetch('/api/auth/logout', { method: 'POST' });
  currentUser.set(null);
  location.href = '/login';
}

export async function api(path, options = {}) {
  const response = await fetch(path, options);
  if (response.status === 401) { currentUser.set(null); location.href = '/login'; throw new Error('Authentication required'); }
  return response;
}
