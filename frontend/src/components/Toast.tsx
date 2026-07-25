import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";
import Button from "./Button";

export type ToastItem = {
  id: string;
  message: string;
  variant?: "info" | "success" | "danger";
  actionLabel?: string;
  onAction?: () => void;
};

type ToastCtx = {
  push: (t: Omit<ToastItem, "id">) => void;
};

const Ctx = createContext<ToastCtx>({ push: () => {} });

export function useToast() {
  return useContext(Ctx);
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);

  const push = useCallback((t: Omit<ToastItem, "id">) => {
    const id = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
    setItems((prev) => [...prev, { ...t, id }]);
    window.setTimeout(() => {
      setItems((prev) => prev.filter((x) => x.id !== id));
    }, 4500);
  }, []);

  const value = useMemo(() => ({ push }), [push]);

  return (
    <Ctx.Provider value={value}>
      {children}
      <div className="toast-host" aria-live="polite">
        {items.map((t) => (
          <div key={t.id} className={`toast toast-${t.variant || "info"}`}>
            <span>{t.message}</span>
            {t.actionLabel && t.onAction && (
              <Button variant="ghost" onClick={t.onAction}>
                {t.actionLabel}
              </Button>
            )}
            <button
              type="button"
              className="toast-dismiss"
              aria-label="Dismiss"
              onClick={() => setItems((prev) => prev.filter((x) => x.id !== t.id))}
            >
              ×
            </button>
          </div>
        ))}
      </div>
    </Ctx.Provider>
  );
}
