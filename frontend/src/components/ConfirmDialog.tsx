import { useEffect, useRef, type ReactNode } from "react";
import Button from "./Button";

export default function ConfirmDialog({
  open,
  title,
  body,
  confirmLabel = "Remove",
  danger = true,
  busy = false,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  title: string;
  body: ReactNode;
  confirmLabel?: string;
  danger?: boolean;
  busy?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);

  if (!open) return null;

  return (
    <dialog
      ref={ref}
      className="confirm-dialog"
      onClose={onCancel}
      onCancel={(e) => {
        e.preventDefault();
        onCancel();
      }}
    >
      <form
        method="dialog"
        className="confirm-dialog-inner"
        onSubmit={(e) => {
          e.preventDefault();
          onConfirm();
        }}
      >
        <h2>{title}</h2>
        <div className="confirm-dialog-body">{body}</div>
        <div className="confirm-dialog-actions">
          <Button variant="secondary" onClick={onCancel} disabled={busy}>
            Cancel
          </Button>
          <Button variant={danger ? "danger" : "primary"} type="submit" disabled={busy}>
            {busy ? "Working…" : confirmLabel}
          </Button>
        </div>
      </form>
    </dialog>
  );
}
