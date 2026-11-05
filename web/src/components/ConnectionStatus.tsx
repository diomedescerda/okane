import { useSyncExternalStore } from "react";

import { useI18n } from "../i18n/I18nContext";
import { connectionAvailable, subscribeToConnection } from "../lib/connection";

export function ConnectionStatus() {
  const { t } = useI18n();
  const online = useSyncExternalStore(
    subscribeToConnection,
    connectionAvailable,
  );
  return online ? null : (
    <p
      role="status"
      className="border-b border-amber-400/25 bg-amber-400/10 px-4 py-3 text-center text-sm text-amber-200"
    >
      {t("connection.unavailable")}
    </p>
  );
}
