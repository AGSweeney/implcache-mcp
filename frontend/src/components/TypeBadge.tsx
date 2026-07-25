import { typeLabel } from "../sourceUi";

export default function TypeBadge({ kind }: { kind: string }) {
  return <span className={`type-badge type-${kind}`}>{typeLabel(kind)}</span>;
}
