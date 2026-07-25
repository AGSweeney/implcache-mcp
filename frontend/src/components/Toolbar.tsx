import type { ReactNode } from "react";

/** Shared filter/search row wrapper used by Sources (and future list pages). */
export default function Toolbar({ children }: { children: ReactNode }) {
  return <div className="sources-toolbar">{children}</div>;
}
