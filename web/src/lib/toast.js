import { writable } from 'svelte/store';

let nextId = 1;
export const toasts = writable([]);

export function pushToast(message, tone = 'info', timeout = 4200) {
  const id = nextId++;
  toasts.update((items) => [...items, { id, message, tone }]);
  if (timeout > 0) {
    setTimeout(() => dismissToast(id), timeout);
  }
  return id;
}

export function dismissToast(id) {
  toasts.update((items) => items.filter((item) => item.id !== id));
}

export const toast = {
  success: (message) => pushToast(message, 'success'),
  error: (message) => pushToast(message, 'error', 6000),
  info: (message) => pushToast(message, 'info')
};
