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

function formatPrice(value: number) {
  return (value / 100).toFixed(2);
}

export function resolveReferencePriceLabel(
  referencePriceLabel: string | null | undefined,
  referencePriceMin: number,
  referencePriceMax: number
) {
  const label = (referencePriceLabel || '').trim();
  if (label) {
    return label;
  }
  if (referencePriceMin <= 0 && referencePriceMax <= 0) {
    return '参考价待补充';
  }
  if (referencePriceMin > 0 && referencePriceMax > 0 && referencePriceMin !== referencePriceMax) {
    return `参考价 ${formatPrice(referencePriceMin)} - ${formatPrice(referencePriceMax)} 元`;
  }

  const value = Math.max(referencePriceMin, referencePriceMax);
  if (value <= 0) {
    return '参考价待补充';
  }
  return `参考价 ${formatPrice(value)} 元`;
}
