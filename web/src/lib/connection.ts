// navigator.onLine can stay true when Wi-Fi has no internet or the API is unreachable.
let reachable = true;
const listeners = new Set<() => void>();

function setReachable(value: boolean) {
  if (value === reachable) return;
  reachable = value;
  for (const listener of listeners) listener();
}

export function connectionAvailable() {
  return navigator.onLine && reachable;
}

export function subscribeToConnection(listener: () => void) {
  listeners.add(listener);
  const online = () => {
    setReachable(true);
    listener();
  };
  const offline = () => {
    setReachable(false);
    listener();
  };
  window.addEventListener("online", online);
  window.addEventListener("offline", offline);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("online", online);
    window.removeEventListener("offline", offline);
  };
}

export async function fetchWithConnectionStatus(
  path: string,
  init?: RequestInit,
) {
  try {
    const response = await fetch(path, init);
    setReachable(true);
    return response;
  } catch (error) {
    if (error instanceof TypeError || !navigator.onLine) setReachable(false);
    throw error;
  }
}
