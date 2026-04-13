export type AppRuntimeKind = 'wails' | 'web';

type WailsEventCallback = (...data: any[]) => void;

type WailsWindow = {
  go?: {
    app?: {
      App?: unknown;
    };
  };
  runtime?: {
    EventsOnMultiple?: (eventName: string, callback: WailsEventCallback, maxCallbacks: number) => () => void;
  };
};

export function hasWailsRuntime(target: unknown): boolean {
  const candidate = target as WailsWindow;

  return Boolean(candidate?.go?.app?.App);
}

export function resolveAppRuntime(target: unknown = window): AppRuntimeKind {
  return hasWailsRuntime(target) ? 'wails' : 'web';
}

export function onWailsEvent(
  eventName: string,
  callback: WailsEventCallback,
  target: unknown = window
): () => void {
  const candidate = target as WailsWindow;
  const subscribe = candidate?.runtime?.EventsOnMultiple;
  if (typeof subscribe !== 'function') {
    return () => {};
  }
  return subscribe(eventName, callback, -1);
}
