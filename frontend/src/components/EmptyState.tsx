import type { ReactNode } from "react";

export default function EmptyState({
  title,
  body,
  action,
  icon,
}: {
  title: string;
  body?: string;
  action?: ReactNode;
  icon?: ReactNode;
}) {
  return (
    <div className="empty-state">
      {icon && <div className="empty-state-icon">{icon}</div>}
      <h3>{title}</h3>
      {body && <p>{body}</p>}
      {action && <div className="empty-state-action">{action}</div>}
    </div>
  );
}
