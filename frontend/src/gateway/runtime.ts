export type AppRuntimeKind = 'wails' | 'web';

export function hasWailsRuntime(target: unknown): boolean {
  const candidate = target as {
    go?: {
      app?: {
        App?: unknown;
      };
    };
  };

  return Boolean(candidate?.go?.app?.App);
}

export function resolveAppRuntime(target: unknown = window): AppRuntimeKind {
  return hasWailsRuntime(target) ? 'wails' : 'web';
}
