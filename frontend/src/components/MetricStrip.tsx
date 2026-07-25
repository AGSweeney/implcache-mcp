export type Metric = { label: string; value: string | number };

export default function MetricStrip({ items }: { items: Metric[] }) {
  return (
    <div className="metric-strip" role="group" aria-label="Summary">
      {items.map((m) => (
        <div className="metric-strip-item" key={m.label}>
          <span className="metric-strip-value">{m.value}</span>
          <span className="metric-strip-label">{m.label}</span>
        </div>
      ))}
    </div>
  );
}
