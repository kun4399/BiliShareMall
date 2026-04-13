/* eslint-disable class-methods-use-this, max-params, @typescript-eslint/no-invalid-void-type */
import { EventsOn } from '~/wailsjs/runtime/runtime';
import {
  CreateScrapyItem as WailsCreateScrapyItem,
  DeleteScrapyItem as WailsDeleteScrapyItem,
  DoneTask as WailsDoneTask,
  GetC2CItemNameBySku as WailsGetC2CItemNameBySku,
  GetLoginKeyAndUrl as WailsGetLoginKeyAndUrl,
  GetMarketRuntimeConfig as WailsGetMarketRuntimeConfig,
  GetMonitorConfig as WailsGetMonitorConfig,
  GetRunningTaskIds as WailsGetRunningTaskIds,
  ListC2CItem as WailsListC2CItem,
  ListC2CItemDetailBySku as WailsListC2CItemDetailBySku,
  ListMonitorRuleHits as WailsListMonitorRuleHits,
  ReadAllScrapyItems as WailsReadAllScrapyItems,
  SaveMonitorConfig as WailsSaveMonitorConfig,
  StartTask as WailsStartTask,
  VerifyLogin as WailsVerifyLogin
} from '~/wailsjs/go/app/App';
import type { auth, catalog, dao, scrapy } from '~/wailsjs/go/models';
import { resolveAppRuntime } from './runtime';

type EventCallback = (payload: unknown) => void;

export interface AppGateway {
  GetLoginKeyAndUrl(): Promise<auth.LoginInfo>;
  VerifyLogin(loginKey: string): Promise<auth.VerifyLoginResponse>;
  ListC2CItem(
    page: number,
    pageSize: number,
    filterName: string,
    sortOption: number,
    startTime: number,
    endTime: number,
    fromPrice: number,
    toPrice: number
  ): Promise<catalog.C2CItemGroupListVO>;
  GetC2CItemNameBySku(skuId: number): Promise<string>;
  ListC2CItemDetailBySku(
    skuId: number,
    page: number,
    pageSize: number,
    sortOption: number,
    statusFilter: string,
    cookie: string
  ): Promise<catalog.C2CItemDetailListVO>;
  ReadAllScrapyItems(): Promise<dao.ScrapyItem[]>;
  DeleteScrapyItem(id: number): Promise<void>;
  CreateScrapyItem(item: dao.ScrapyItem): Promise<number>;
  StartTask(taskID: number, cookies: string): Promise<void>;
  DoneTask(taskID: number): Promise<void>;
  GetRunningTaskIds(): Promise<number[]>;
  GetMarketRuntimeConfig(cookieStr: string): Promise<scrapy.MarketRuntimeConfig>;
  GetMonitorConfig(): Promise<scrapy.MonitorConfig>;
  SaveMonitorConfig(config: scrapy.MonitorConfig): Promise<void>;
  ListMonitorRuleHits(limitPerRule: number): Promise<scrapy.MonitorHitGroup[]>;
  OnEvent(eventName: string, callback: EventCallback): () => void;
}

class WailsGateway implements AppGateway {
  GetLoginKeyAndUrl() {
    return WailsGetLoginKeyAndUrl();
  }

  VerifyLogin(loginKey: string) {
    return WailsVerifyLogin(loginKey);
  }

  ListC2CItem(
    page: number,
    pageSize: number,
    filterName: string,
    sortOption: number,
    startTime: number,
    endTime: number,
    fromPrice: number,
    toPrice: number
  ) {
    return WailsListC2CItem(page, pageSize, filterName, sortOption, startTime, endTime, fromPrice, toPrice);
  }

  GetC2CItemNameBySku(skuId: number) {
    return WailsGetC2CItemNameBySku(skuId);
  }

  ListC2CItemDetailBySku(
    skuId: number,
    page: number,
    pageSize: number,
    sortOption: number,
    statusFilter: string,
    cookie: string
  ) {
    return WailsListC2CItemDetailBySku(skuId, page, pageSize, sortOption, statusFilter, cookie);
  }

  ReadAllScrapyItems() {
    return WailsReadAllScrapyItems();
  }

  DeleteScrapyItem(id: number) {
    return WailsDeleteScrapyItem(id);
  }

  async CreateScrapyItem(item: dao.ScrapyItem) {
    return Number(await WailsCreateScrapyItem(item));
  }

  StartTask(taskID: number, cookies: string) {
    return WailsStartTask(taskID, cookies);
  }

  DoneTask(taskID: number) {
    return WailsDoneTask(taskID);
  }

  GetRunningTaskIds() {
    return WailsGetRunningTaskIds();
  }

  GetMarketRuntimeConfig(cookieStr: string) {
    return WailsGetMarketRuntimeConfig(cookieStr);
  }

  GetMonitorConfig() {
    return WailsGetMonitorConfig();
  }

  SaveMonitorConfig(config: scrapy.MonitorConfig) {
    return WailsSaveMonitorConfig(config);
  }

  ListMonitorRuleHits(limitPerRule: number) {
    return WailsListMonitorRuleHits(limitPerRule);
  }

  OnEvent(eventName: string, callback: EventCallback) {
    return EventsOn(eventName, callback);
  }
}

class WebEventBridge {
  private source: EventSource | null = null;
  private listeners = new Map<string, Set<EventCallback>>();
  private handlers = new Map<string, (event: MessageEvent) => void>();

  subscribe(eventName: string, callback: EventCallback) {
    this.ensureSource();

    let listeners = this.listeners.get(eventName);
    if (!listeners) {
      listeners = new Set<EventCallback>();
      this.listeners.set(eventName, listeners);
      this.attachHandler(eventName);
    }

    listeners.add(callback);

    return () => {
      const currentListeners = this.listeners.get(eventName);
      if (!currentListeners) return;
      currentListeners.delete(callback);
      if (currentListeners.size === 0) {
        this.detachHandler(eventName);
        this.listeners.delete(eventName);
      }
      if (this.listeners.size === 0) {
        this.close();
      }
    };
  }

  private ensureSource() {
    if (this.source) return;
    this.source = new EventSource('/api/events');
    this.source.onerror = () => {
      // Let EventSource handle reconnects automatically.
    };
  }

  private attachHandler(eventName: string) {
    if (!this.source || this.handlers.has(eventName)) return;

    const handler = (event: MessageEvent) => {
      const payload = parseEventPayload(event.data);
      const listeners = this.listeners.get(eventName);
      listeners?.forEach(listener => listener(payload));
    };
    this.handlers.set(eventName, handler);
    this.source.addEventListener(eventName, handler as EventListener);
  }

  private detachHandler(eventName: string) {
    if (!this.source) return;
    const handler = this.handlers.get(eventName);
    if (!handler) return;
    this.source.removeEventListener(eventName, handler as EventListener);
    this.handlers.delete(eventName);
  }

  private close() {
    this.handlers.forEach((handler, eventName) => {
      this.source?.removeEventListener(eventName, handler as EventListener);
    });
    this.handlers.clear();
    this.source?.close();
    this.source = null;
  }
}

class WebGateway implements AppGateway {
  private events = new WebEventBridge();

  GetLoginKeyAndUrl() {
    return fetchJSON<auth.LoginInfo>('/api/auth/qr');
  }

  VerifyLogin(loginKey: string) {
    const query = new URLSearchParams({ key: loginKey });
    return fetchJSON<auth.VerifyLoginResponse>(`/api/auth/poll?${query.toString()}`);
  }

  ListC2CItem(
    page: number,
    pageSize: number,
    filterName: string,
    sortOption: number,
    startTime: number,
    endTime: number,
    fromPrice: number,
    toPrice: number
  ) {
    const query = new URLSearchParams({
      page: String(page),
      pageSize: String(pageSize),
      keyword: filterName,
      sortOption: String(sortOption),
      startTime: String(startTime),
      endTime: String(endTime),
      fromPrice: String(fromPrice),
      toPrice: String(toPrice)
    });
    return fetchJSON<catalog.C2CItemGroupListVO>(`/api/catalog/items?${query.toString()}`);
  }

  async GetC2CItemNameBySku(skuId: number) {
    const result = await fetchJSON<{ name: string }>(`/api/catalog/sku/${skuId}/name`);
    return result.name || '';
  }

  ListC2CItemDetailBySku(
    skuId: number,
    page: number,
    pageSize: number,
    sortOption: number,
    statusFilter: string,
    cookie: string
  ) {
    const query = new URLSearchParams({
      page: String(page),
      pageSize: String(pageSize),
      sortOption: String(sortOption),
      statusFilter
    });
    return fetchJSON<catalog.C2CItemDetailListVO>(`/api/catalog/items/${skuId}?${query.toString()}`, {
      headers: cookieHeader(cookie)
    });
  }

  ReadAllScrapyItems() {
    return fetchJSON<dao.ScrapyItem[]>('/api/scrapy/tasks');
  }

  DeleteScrapyItem(id: number) {
    return fetchJSON<void>(`/api/scrapy/tasks/${id}`, { method: 'DELETE' });
  }

  async CreateScrapyItem(item: dao.ScrapyItem) {
    const result = await fetchJSON<{ id: number }>('/api/scrapy/tasks', {
      method: 'POST',
      body: JSON.stringify(item)
    });
    return Number(result.id || 0);
  }

  StartTask(taskID: number, cookies: string) {
    return fetchJSON<void>(`/api/scrapy/tasks/${taskID}/start`, {
      method: 'POST',
      headers: cookieHeader(cookies)
    });
  }

  DoneTask(taskID: number) {
    return fetchJSON<void>(`/api/scrapy/tasks/${taskID}/stop`, { method: 'POST' });
  }

  GetRunningTaskIds() {
    return fetchJSON<number[]>('/api/scrapy/running-task-ids');
  }

  GetMarketRuntimeConfig(cookieStr: string) {
    return fetchJSON<scrapy.MarketRuntimeConfig>('/api/scrapy/runtime-config', {
      headers: cookieHeader(cookieStr)
    });
  }

  GetMonitorConfig() {
    return fetchJSON<scrapy.MonitorConfig>('/api/monitor/config');
  }

  SaveMonitorConfig(config: scrapy.MonitorConfig) {
    return fetchJSON<void>('/api/monitor/config', {
      method: 'PUT',
      body: JSON.stringify(config)
    });
  }

  ListMonitorRuleHits(limitPerRule: number) {
    const query = new URLSearchParams({ limitPerRule: String(limitPerRule) });
    return fetchJSON<scrapy.MonitorHitGroup[]>(`/api/monitor/rule-hits?${query.toString()}`);
  }

  OnEvent(eventName: string, callback: EventCallback) {
    return this.events.subscribe(eventName, callback);
  }
}

function cookieHeader(cookie: string): HeadersInit | undefined {
  if (!cookie) return undefined;
  return {
    'X-Bili-Cookie': cookie
  };
}

async function fetchJSON<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T> {
  const response = await fetch(input, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers || {})
    }
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  const payload = text ? JSON.parse(text) : null;

  if (!response.ok) {
    const message = payload?.message || `request failed with status ${response.status}`;
    throw new Error(message);
  }

  return payload as T;
}

function parseEventPayload(payload: string) {
  if (!payload) return payload;
  try {
    return JSON.parse(payload);
  } catch {
    return payload;
  }
}

function createGateway(): AppGateway {
  return resolveAppRuntime() === 'wails' ? new WailsGateway() : new WebGateway();
}

export const appGateway = createGateway();

export const GetLoginKeyAndUrl = () => appGateway.GetLoginKeyAndUrl();
export const VerifyLogin = (loginKey: string) => appGateway.VerifyLogin(loginKey);
export const ListC2CItem = (
  page: number,
  pageSize: number,
  filterName: string,
  sortOption: number,
  startTime: number,
  endTime: number,
  fromPrice: number,
  toPrice: number
) => appGateway.ListC2CItem(page, pageSize, filterName, sortOption, startTime, endTime, fromPrice, toPrice);
export const GetC2CItemNameBySku = (skuId: number) => appGateway.GetC2CItemNameBySku(skuId);
export const ListC2CItemDetailBySku = (
  skuId: number,
  page: number,
  pageSize: number,
  sortOption: number,
  statusFilter: string,
  cookie: string
) => appGateway.ListC2CItemDetailBySku(skuId, page, pageSize, sortOption, statusFilter, cookie);
export const ReadAllScrapyItems = () => appGateway.ReadAllScrapyItems();
export const DeleteScrapyItem = (id: number) => appGateway.DeleteScrapyItem(id);
export const CreateScrapyItem = (item: dao.ScrapyItem) => appGateway.CreateScrapyItem(item);
export const StartTask = (taskID: number, cookies: string) => appGateway.StartTask(taskID, cookies);
export const DoneTask = (taskID: number) => appGateway.DoneTask(taskID);
export const GetRunningTaskIds = () => appGateway.GetRunningTaskIds();
export const GetMarketRuntimeConfig = (cookieStr: string) => appGateway.GetMarketRuntimeConfig(cookieStr);
export const GetMonitorConfig = () => appGateway.GetMonitorConfig();
export const SaveMonitorConfig = (config: scrapy.MonitorConfig) => appGateway.SaveMonitorConfig(config);
export const ListMonitorRuleHits = (limitPerRule: number) => appGateway.ListMonitorRuleHits(limitPerRule);
export const OnAppEvent = (eventName: string, callback: EventCallback) => appGateway.OnEvent(eventName, callback);
