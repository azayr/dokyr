import { writable } from 'svelte/store';

export const currentUser = writable(null);

// Permissions held by the signed-in account. Used only to hide controls the
// caller cannot use; every route enforces its own permission on the server.
export const currentPermissions = writable([]);

export async function loadSession() {
  const response = await fetch('/api/auth/me');
  if (!response.ok) { currentUser.set(null); currentPermissions.set([]); return null; }
  const data = await response.json();
  currentUser.set(data.user);
  currentPermissions.set(data.permissions || []);
  return data.user;
}

export function can(permissions, permission) {
  return Array.isArray(permissions) && permissions.includes(permission);
}

export async function logout() {
  await fetch('/api/auth/logout', { method: 'POST' });
  currentUser.set(null);
  currentPermissions.set([]);
  location.href = '/login';
}

export async function api(path, options = {}) {
  const response = await fetch(path, options);
  if (response.status === 401) { currentUser.set(null); location.href = '/login'; throw new Error('Authentication required'); }
  return response;
}
