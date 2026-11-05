import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";

import type { AccountType, InstitutionType, MatchField, MatchType } from "../api/finance";
import { useAuth } from "../auth/AuthContext";
import { setFormatLanguage } from "../lib/format";
import {
  LANGUAGE_STORAGE_KEY,
  normalizeLanguage,
  resolveInitialLanguage,
  translate,
  translateAccountType,
  translateApiError,
  translateCategoryName,
  translateInstitutionType,
  translateMatchField,
  translateMatchType,
} from "../lib/i18n";
import type { Language, TranslationKey } from "../lib/i18n";

export interface I18nState {
  language: Language;
  setLanguage: (lang: Language) => void;
  t: (key: TranslationKey, params?: Record<string, string | number>) => string;
  tCategory: (slug?: string | null, fallbackName?: string | null) => string;
  tAccountType: (type: AccountType) => string;
  tInstitutionType: (type: InstitutionType) => string;
  tMatchType: (type: MatchType) => string;
  tMatchField: (field: MatchField) => string;
  tError: (err: unknown, fallbackKey?: TranslationKey) => string;
}

const I18nContext = createContext<I18nState | null>(null);

function readStoredLanguage(): string | null {
  try {
    return window.localStorage.getItem(LANGUAGE_STORAGE_KEY);
  } catch {
    return null;
  }
}

function writeStoredLanguage(lang: Language): void {
  try {
    window.localStorage.setItem(LANGUAGE_STORAGE_KEY, lang);
  } catch {
    // Ignore storage write errors in private browsing modes.
  }
}

function detectInitialLanguage(): Language {
  const stored = typeof window !== "undefined" ? readStoredLanguage() : null;
  const browserLanguages =
    typeof navigator !== "undefined"
      ? navigator.languages?.length
        ? navigator.languages
        : navigator.language
          ? [navigator.language]
          : []
      : [];
  const resolved = resolveInitialLanguage({ stored, browserLanguages });
  setFormatLanguage(resolved);
  return resolved;
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const { user, updateLanguage } = useAuth();
  const [language, setLanguageState] = useState<Language>(detectInitialLanguage);
  const lastUserIdRef = useRef<string | null>(null);
  const explicitPreAuthChoiceRef = useRef(false);

  // Ensure format helpers always use the active language during render.
  setFormatLanguage(language);

  useEffect(() => {
    document.documentElement.lang = language;
    writeStoredLanguage(language);
  }, [language]);

  useEffect(() => {
    if (!user) {
      lastUserIdRef.current = null;
      return;
    }
    if (lastUserIdRef.current === user.id) {
      return;
    }
    lastUserIdRef.current = user.id;
    const serverLang = normalizeLanguage(user.language);

    if (explicitPreAuthChoiceRef.current) {
      explicitPreAuthChoiceRef.current = false;
      if (serverLang !== language) {
        void updateLanguage(language).catch(() => {});
      }
      return;
    }

    if (serverLang && serverLang !== language) {
      setFormatLanguage(serverLang);
      setLanguageState(serverLang);
      writeStoredLanguage(serverLang);
    }
  }, [user, language, updateLanguage]);

  const setLanguage = useCallback(
    (next: Language) => {
      setFormatLanguage(next);
      setLanguageState(next);
      writeStoredLanguage(next);
      if (user) {
        void updateLanguage(next).catch(() => {});
      } else {
        explicitPreAuthChoiceRef.current = true;
      }
    },
    [user, updateLanguage],
  );

  const value = useMemo<I18nState>(
    () => ({
      language,
      setLanguage,
      t: (key, params) => translate(language, key, params),
      tCategory: (slug, fallbackName) => translateCategoryName(language, slug, fallbackName),
      tAccountType: (type) => translateAccountType(language, type),
      tInstitutionType: (type) => translateInstitutionType(language, type),
      tMatchType: (type) => translateMatchType(language, type),
      tMatchField: (field) => translateMatchField(language, field),
      tError: (err, fallbackKey) => translateApiError(language, err, fallbackKey),
    }),
    [language, setLanguage],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useI18n(): I18nState {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used inside <I18nProvider>");
  return ctx;
}
