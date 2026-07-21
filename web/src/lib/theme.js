import { writable } from 'svelte/store';

const STORAGE_KEY = 'dokyr-theme';
const LEGACY_KEY = 'selfhost-theme';
const MODES = ['light', 'dark', 'system'];

export const themeMode = writable('light');
export const resolvedTheme = writable('light');

function systemPrefersDark() {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function apply(mode) {
  if (typeof document === 'undefined') return;
  const dark = mode === 'dark' || (mode === 'system' && systemPrefersDark());
  document.documentElement.classList.toggle('theme-dark', dark);
  const themeColor = document.querySelector('meta[name="theme-color"]');
  if (themeColor) themeColor.setAttribute('content', dark ? '#0c1117' : '#f5f7fa');
  resolvedTheme.set(dark ? 'dark' : 'light');
}

export function initTheme() {
  if (typeof window === 'undefined') return 'light';
  let stored = null;
  try {
    stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) {
      const legacy = localStorage.getItem(LEGACY_KEY);
      if (legacy === 'dark' || legacy === 'light') {
        stored = legacy;
        localStorage.setItem(STORAGE_KEY, stored);
        localStorage.removeItem(LEGACY_KEY);
      }
    }
  } catch {}
  const mode = MODES.includes(stored) ? stored : 'light';
  themeMode.set(mode);
  apply(mode);
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    themeMode.subscribe((current) => apply(current))();
  });
  window.addEventListener('storage', (event) => {
    if (event.key === STORAGE_KEY && MODES.includes(event.newValue)) {
      themeMode.set(event.newValue);
      apply(event.newValue);
    }
  });
  return mode;
}

export function setTheme(mode) {
  if (!MODES.includes(mode)) return;
  try {
    localStorage.setItem(STORAGE_KEY, mode);
  } catch {}
  themeMode.set(mode);
  apply(mode);
}

export function cycleTheme(current) {
  const next = current === 'light' ? 'dark' : current === 'dark' ? 'system' : 'light';
  setTheme(next);
  return next;
}
