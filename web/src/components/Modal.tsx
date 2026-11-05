import { useEffect, useId, useRef, type ReactNode } from "react";

import { useI18n } from "../i18n/I18nContext";

interface ModalProps {
  title: string;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
}

export function Modal({ title, onClose, children, footer }: ModalProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const { t } = useI18n();

  useEffect(() => {
    const dialog = dialogRef.current!;
    const returnFocus = document.activeElement;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const viewport = window.visualViewport;
    function sizeToViewport() {
      dialog.style.setProperty(
        "--dialog-viewport-height",
        `${viewport?.height ?? window.innerHeight}px`,
      );
      dialog.style.setProperty(
        "--dialog-offset-top",
        `${viewport?.offsetTop ?? 0}px`,
      );
    }
    sizeToViewport();
    viewport?.addEventListener("resize", sizeToViewport);
    viewport?.addEventListener("scroll", sizeToViewport);
    window.addEventListener("resize", sizeToViewport);
    dialog.showModal();
    return () => {
      viewport?.removeEventListener("resize", sizeToViewport);
      viewport?.removeEventListener("scroll", sizeToViewport);
      window.removeEventListener("resize", sizeToViewport);
      dialog.close();
      document.body.style.overflow = previousOverflow;
      if (returnFocus instanceof HTMLElement && returnFocus.isConnected) {
        returnFocus.focus({ preventScroll: true });
      }
    };
  }, []);

  return (
    <dialog
      ref={dialogRef}
      aria-labelledby={titleId}
      className="app-dialog"
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="flex shrink-0 items-center justify-between gap-3 border-b border-slate-800 px-5 py-3">
        <h2 id={titleId} className="min-w-0 break-words font-medium">
          {title}
        </h2>
        <button
          type="button"
          onClick={onClose}
          className="btn-ghost size-11 shrink-0 text-2xl"
          aria-label={t("common.close")}
        >
          ×
        </button>
      </div>
      <div className="min-h-0 overflow-y-auto overscroll-contain px-5 py-4">
        {children}
      </div>
      {footer && (
        <div className="flex shrink-0 flex-wrap justify-end gap-2 border-t border-slate-800 px-5 py-4">
          {footer}
        </div>
      )}
    </dialog>
  );
}
