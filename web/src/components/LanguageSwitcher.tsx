import { useI18n } from "../i18n/I18nContext";
import { LANGUAGE_LABELS, SUPPORTED_LANGUAGES } from "../lib/i18n";

export function LanguageSwitcher({ className = "" }: { className?: string }) {
  const { language, setLanguage, t } = useI18n();

  return (
    <div
      role="group"
      aria-label={t("common.language")}
      className={`inline-flex items-center rounded-xl border border-slate-800/90 bg-slate-900/75 p-0.5 text-xs backdrop-blur-sm sm:p-1 ${className}`.trim()}
    >
      {SUPPORTED_LANGUAGES.map((lang) => {
        const info = LANGUAGE_LABELS[lang];
        const active = language === lang;
        return (
          <button
            key={lang}
            type="button"
            data-lang={lang}
            aria-label={info.native}
            aria-pressed={active}
            onClick={() => setLanguage(lang)}
            className={`min-h-8 rounded-lg px-2 py-1 font-medium transition sm:px-2.5 ${
              active
                ? "bg-emerald-400/15 text-emerald-300 shadow-xs shadow-black/30"
                : "text-slate-400 hover:bg-slate-800/70 hover:text-slate-100"
            }`}
          >
            <span className="sm:hidden">{info.short}</span>
            <span className="hidden sm:inline">{info.native}</span>
          </button>
        );
      })}
    </div>
  );
}
