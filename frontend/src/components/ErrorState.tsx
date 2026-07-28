import type { ReactNode } from "react";
import Button from "./Button";

function DefaultErrorIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M12 8v5M12 16.5h.01" strokeLinecap="round" />
      <path d="M10.3 4.8 2.9 18a2 2 0 0 0 1.7 3h14.8a2 2 0 0 0 1.7-3L13.7 4.8a2 2 0 0 0-3.4 0Z" strokeLinejoin="round" />
    </svg>
  );
}

export default function ErrorState({
  message,
  onRetry,
  icon,
}: {
  message: string;
  onRetry?: () => void;
  icon?: ReactNode;
}) {
  return (
    <div className="error-state" role="alert">
      <div className="error-state-icon">{icon ?? <DefaultErrorIcon />}</div>
      <p>{message}</p>
      {onRetry && (
        <Button variant="secondary" onClick={onRetry}>
          Retry
        </Button>
      )}
    </div>
  );
}
