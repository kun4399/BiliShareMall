import { ListC2CItem, ListC2CItemDetailBySku } from '~/wailsjs/go/app/App';

export interface CatalogListQuery {
  page: number;
  pageSize: number;
  keyword: string;
  sortOption: number;
  startTime: number;
  endTime: number;
  fromPrice: number;
  toPrice: number;
}

export interface CatalogDetailQuery {
  skuId: number;
  page: number;
  pageSize: number;
  sortOption: number;
  statusFilter: string;
  cookie: string;
}

export function fetchCatalogList(query: CatalogListQuery) {
  return ListC2CItem(
    query.page,
    query.pageSize,
    query.keyword,
    query.sortOption,
    query.startTime,
    query.endTime,
    query.fromPrice,
    query.toPrice
  );
}

export function fetchCatalogDetail(query: CatalogDetailQuery) {
  return ListC2CItemDetailBySku(
    query.skuId,
    query.page,
    query.pageSize,
    query.sortOption,
    query.statusFilter,
    query.cookie
  );
}
