import { resolveAppRuntime } from '@/gateway/runtime';

function absoluteImageURL(url: string) {
  if (!url) {
    return '';
  }
  if (url.startsWith('//')) {
    return `https:${url}`;
  }
  return url;
}

export function normalizeImage(url: string) {
  const resolved = absoluteImageURL(url);
  if (!resolved) {
    return '';
  }
  if (resolveAppRuntime() === 'wails') {
    return resolved;
  }
  return `/api/assets/image?url=${encodeURIComponent(resolved)}`;
}
