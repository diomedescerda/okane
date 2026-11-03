import { useSyncExternalStore } from "react";

import { connectionAvailable, subscribeToConnection } from "../lib/connection";

export function ConnectionStatus() {
  const online = useSyncExternalStore(
    subscribeToConnection,
    connectionAvailable,
  );
  return online ? null : (
    <p
      role="status"
      className="border-b border-amber-400/25 bg-amber-400/10 px-4 py-3 text-center text-sm text-amber-200"
    >
      Connection unavailable. Reconnect to load your accounts or save changes.
    </p>
  );
}
