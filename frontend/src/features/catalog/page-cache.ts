import type { catalog } from '~/wailsjs/go/models';
import type { CatalogListQuery } from './api';

interface CacheEntry {
  value: catalog.C2CItemGroupListVO;
  cachedAt: number;
}

export interface CatalogRequestToken {
  sequence: number;
  epoch: number;
}

export class CatalogPageCache {
  private readonly entries = new Map<string, CacheEntry>();

  constructor(private readonly maxEntries: number = 12) {}

  get(key: string) {
    const entry = this.entries.get(key);
    if (!entry) return null;
    this.entries.delete(key);
    this.entries.set(key, entry);
    return entry;
  }

  set(key: string, value: catalog.C2CItemGroupListVO) {
    this.entries.delete(key);
    this.entries.set(key, { value, cachedAt: Date.now() });
    while (this.entries.size > this.maxEntries) {
      const oldestKey = this.entries.keys().next().value as string | undefined;
      if (!oldestKey) break;
      this.entries.delete(oldestKey);
    }
  }

  clear() {
    this.entries.clear();
  }

  get size() {
    return this.entries.size;
  }
}

export class CatalogRequestCoordinator {
  private sequence = 0;
  private epoch = 0;

  start(): CatalogRequestToken {
    return { sequence: ++this.sequence, epoch: this.epoch };
  }

  snapshot(): CatalogRequestToken {
    return { sequence: this.sequence, epoch: this.epoch };
  }

  isCurrent(token: CatalogRequestToken) {
    return token.sequence === this.sequence && token.epoch === this.epoch;
  }

  invalidate() {
    this.epoch += 1;
  }
}

export function getAdjacentCatalogPages(page: number, pageCount: number) {
  return [page - 1, page + 1].filter(candidate => candidate >= 1 && candidate <= pageCount);
}

export function createCatalogQueryKey(query: CatalogListQuery) {
  return [
    query.page,
    query.pageSize,
    query.keyword.trim(),
    query.sortOption,
    query.startTime,
    query.endTime,
    query.fromPrice,
    query.toPrice
  ].join('|');
}

export const catalogPageCache = new CatalogPageCache(12);
