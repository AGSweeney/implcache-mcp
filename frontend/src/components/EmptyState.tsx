import type { ReactNode } from "react";

export default function EmptyState({
  title,
  body,
  action,
}: {
  title: string;
  body?: string;
  action?: ReactNode;
}) {
  return (
    <div className="empty-state">
      <h3>{title}</h3>
      {body && <p>{body}</p>}
      {action && <div className="empty-state-action">{action}</div>}
    </div>
  );
}
