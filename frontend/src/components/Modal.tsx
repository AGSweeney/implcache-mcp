import { useEffect, type ReactNode } from "react";
import { createPortal } from "react-dom";
import Button from "./Button";

export default function Modal({
  open,
  title,
  onClose,
  children,
  sticky,
  headerActions,
  wide = false,
  fullscreen = false,
  onToggleFullscreen,
}: {
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
  /** Non-scrolling chrome below the title (metadata, tabs). */
  sticky?: ReactNode;
  headerActions?: ReactNode;
  wide?: boolean;
  fullscreen?: boolean;
  onToggleFullscreen?: () => void;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        if (fullscreen && onToggleFullscreen) onToggleFullscreen();
        else onClose();
      }
    };
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    document.addEventListener("keydown", onKey);
    return () => {
      document.body.style.overflow = prev;
      document.removeEventListener("keydown", onKey);
    };
  }, [open, onClose, fullscreen, onToggleFullscreen]);

  if (!open) return null;

  return createPortal(
    <div
      className={["modal-backdrop", fullscreen ? "modal-backdrop-fullscreen" : ""].filter(Boolean).join(" ")}
      onClick={onClose}
      role="presentation"
    >
      <div
        className={[
          "modal",
          wide ? "modal-wide" : "",
          sticky ? "modal-split" : "",
          fullscreen ? "modal-fullscreen" : "",
        ]
          .filter(Boolean)
          .join(" ")}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onClick={(e) => e.stopPropagation()}
      >
        <header className="modal-head">
          <h2>{title}</h2>
          <div className="modal-head-actions">
            {headerActions}
            {onToggleFullscreen && (
              <Button
                variant="icon"
                className="modal-fs-btn"
                aria-label={fullscreen ? "Exit full screen" : "Full screen"}
                title={fullscreen ? "Exit full screen" : "Full screen"}
                onClick={onToggleFullscreen}
              >
                {fullscreen ? "⤡" : "⤢"}
              </Button>
            )}
            <Button variant="icon" aria-label="Close" onClick={onClose}>
              ×
            </Button>
          </div>
        </header>
        {sticky && <div className="modal-sticky">{sticky}</div>}
        <div className="modal-body">{children}</div>
      </div>
    </div>,
    document.body,
  );
}
