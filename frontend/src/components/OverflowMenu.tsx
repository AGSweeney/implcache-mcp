import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import Button from "./Button";

export type MenuItem = {
  label: string;
  onClick: () => void;
  danger?: boolean;
  disabled?: boolean;
};

type Pos = { top: number; left: number };

export default function OverflowMenu({ items, label = "More actions" }: { items: MenuItem[]; label?: string }) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<Pos>({ top: 0, left: 0 });
  const id = useId();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLUListElement>(null);

  useLayoutEffect(() => {
    if (!open || !triggerRef.current) return;
    const place = () => {
      const trigger = triggerRef.current;
      const menu = menuRef.current;
      if (!trigger) return;
      const r = trigger.getBoundingClientRect();
      const mw = menu?.offsetWidth ?? 176;
      const mh = menu?.offsetHeight ?? 120;
      const gap = 4;
      let top = r.bottom + gap;
      let left = r.right - mw;
      if (top + mh > window.innerHeight - 8) {
        top = Math.max(8, r.top - mh - gap);
      }
      if (left < 8) left = 8;
      if (left + mw > window.innerWidth - 8) left = window.innerWidth - mw - 8;
      setPos({ top, left });
    };
    place();
    // second pass after menu mounts with real size
    requestAnimationFrame(place);
    window.addEventListener("resize", place);
    window.addEventListener("scroll", place, true);
    return () => {
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", place, true);
    };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      const t = e.target as Node;
      if (triggerRef.current?.contains(t) || menuRef.current?.contains(t)) return;
      setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div className="overflow-menu">
      <Button
        ref={triggerRef}
        variant="icon"
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen((v) => !v)}
      >
        ···
      </Button>
      {open &&
        createPortal(
          <ul
            className="overflow-menu-list overflow-menu-portal"
            id={id}
            role="menu"
            ref={menuRef}
            style={{ top: pos.top, left: pos.left }}
          >
            {items.map((item) => (
              <li key={item.label} role="none">
                <button
                  type="button"
                  role="menuitem"
                  className={item.danger ? "danger" : undefined}
                  disabled={item.disabled}
                  onClick={() => {
                    setOpen(false);
                    item.onClick();
                  }}
                >
                  {item.label}
                </button>
              </li>
            ))}
          </ul>,
          document.body,
        )}
    </div>
  );
}
