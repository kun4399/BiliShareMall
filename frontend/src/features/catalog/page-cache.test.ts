import assert from 'node:assert/strict';
import test from 'node:test';
import { catalog } from '~/wailsjs/go/models';
import {
  CatalogPageCache,
  CatalogRequestCoordinator,
  createCatalogQueryKey,
  getAdjacentCatalogPages
} from './page-cache';

function page(currentPage: number): catalog.C2CItemGroupListVO {
  return catalog.C2CItemGroupListVO.createFrom({
    items: [],
    total: 30,
    totalPages: 3,
    currentPage
  });
}

test('catalog query key changes for pagination and filters', () => {
  const base = {
    page: 1,
    pageSize: 12,
    keyword: '',
    sortOption: 1,
    startTime: -1,
    endTime: -1,
    fromPrice: -1,
    toPrice: -1
  };

  assert.notEqual(createCatalogQueryKey(base), createCatalogQueryKey({ ...base, page: 2 }));
  assert.notEqual(createCatalogQueryKey(base), createCatalogQueryKey({ ...base, keyword: '徽章' }));
  assert.equal(createCatalogQueryKey({ ...base, keyword: ' 徽章 ' }), createCatalogQueryKey({ ...base, keyword: '徽章' }));
});

test('catalog page cache evicts the least recently used page', () => {
  const cache = new CatalogPageCache(2);
  cache.set('page-1', page(1));
  cache.set('page-2', page(2));
  assert.equal(cache.get('page-1')?.value.currentPage, 1);

  cache.set('page-3', page(3));

  assert.equal(cache.get('page-2'), null);
  assert.equal(cache.get('page-1')?.value.currentPage, 1);
  assert.equal(cache.get('page-3')?.value.currentPage, 3);
});

test('clearing catalog cache removes every page', () => {
  const cache = new CatalogPageCache(4);
  cache.set('page-1', page(1));
  cache.set('page-2', page(2));
  cache.clear();
  assert.equal(cache.size, 0);
});

test('adjacent prefetch stays inside the available page range', () => {
  assert.deepEqual(getAdjacentCatalogPages(1, 5), [2]);
  assert.deepEqual(getAdjacentCatalogPages(3, 5), [2, 4]);
  assert.deepEqual(getAdjacentCatalogPages(5, 5), [4]);
});

test('newer requests and event invalidation reject stale responses', () => {
  const coordinator = new CatalogRequestCoordinator();
  const first = coordinator.start();
  const second = coordinator.start();
  assert.equal(coordinator.isCurrent(first), false);
  assert.equal(coordinator.isCurrent(second), true);

  coordinator.invalidate();
  assert.equal(coordinator.isCurrent(second), false);

  const afterInvalidation = coordinator.start();
  assert.equal(coordinator.isCurrent(afterInvalidation), true);
});
