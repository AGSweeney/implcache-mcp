export default function LoadingSkeleton({ rows = 6 }: { rows?: number }) {
  return (
    <div className="skeleton-table" aria-busy="true" aria-label="Loading">
      {Array.from({ length: rows }).map((_, i) => (
        <div className="skeleton-row" key={i}>
          <div className="skeleton-block wide" />
          <div className="skeleton-block" />
          <div className="skeleton-block" />
          <div className="skeleton-block narrow" />
        </div>
      ))}
    </div>
  );
}
