const storagePrefix = import.meta.env.VITE_STORAGE_PREFIX || '';

function createStorage<T extends object>(type: 'local' | 'session') {
  const storage = type === 'session' ? window.sessionStorage : window.localStorage;

  return {
    set<K extends keyof T>(key: K, value: T[K]) {
      storage.setItem(`${storagePrefix}${String(key)}`, JSON.stringify(value));
    },
    get<K extends keyof T>(key: K): T[K] | null {
      const storageKey = `${storagePrefix}${String(key)}`;
      const raw = storage.getItem(storageKey);
      if (!raw) return null;
      try {
        return JSON.parse(raw) as T[K];
      } catch {
        storage.removeItem(storageKey);
        return null;
      }
    },
    remove(key: keyof T) {
      storage.removeItem(`${storagePrefix}${String(key)}`);
    },
    clear() {
      storage.clear();
    }
  };
}

export const localStg = createStorage<StorageType.Local>('local');

export const sessionStg = createStorage<StorageType.Session>('session');
