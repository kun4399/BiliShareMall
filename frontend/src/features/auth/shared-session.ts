interface WaitForSharedLoginSessionOptions {
  attempts?: number;
  delayMs?: number;
}

interface SharedLoginSessionLike {
  loggedIn: boolean;
  updatedAt: number;
}

function sleep(ms: number) {
  if (ms <= 0) {
    return Promise.resolve();
  }
  return new Promise(resolve => {
    setTimeout(resolve, ms);
  });
}

export async function waitForSharedLoginSession(
  getSession: () => Promise<SharedLoginSessionLike>,
  options: WaitForSharedLoginSessionOptions = {}
) {
  const {
    attempts = 5,
    delayMs = 250
  } = options;

  for (let attempt = 0; attempt < attempts; attempt += 1) {
    const session = await getSession();
    if (session.loggedIn) {
      return true;
    }
    if (attempt < attempts - 1) {
      await sleep(delayMs);
    }
  }

  return false;
}
