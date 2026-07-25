import type { ReactNode } from "react";
import type { UiStatus } from "../sourceUi";

type Variant = "success" | "warning" | "danger" | "neutral" | "info" | UiStatus;

const variantClass: Record<string, string> = {
  success: "badge-success",
  warning: "badge-warning",
  danger: "badge-danger",
  neutral: "badge-neutral",
  info: "badge-info",
  ready: "badge-success",
  never: "badge-neutral",
  refreshing: "badge-info",
  attention: "badge-warning",
  failed: "badge-danger",
  disabled: "badge-neutral",
  unknown: "badge-neutral",
  ok: "badge-success",
  err: "badge-danger",
  warn: "badge-warning",
};

export default function StatusBadge({
  children,
  variant = "neutral",
  title,
  showIcon = true,
}: {
  children: ReactNode;
  variant?: Variant;
  title?: string;
  showIcon?: boolean;
}) {
  return (
    <span className={`status-badge ${variantClass[variant] || "badge-neutral"}`} title={title}>
      {showIcon && <span className="status-badge-icon" aria-hidden="true" />}
      {children}
    </span>
  );
}
