/** Format byte counts as B / KB / MB / GB with sensible precision. */
export function formatBytes(n: number | null | undefined): string {
  if (n == null || Number.isNaN(n)) return "—";
  let v = Number(n);
  if (!Number.isFinite(v) || v < 0) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"] as const;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  const digits = i === 0 ? 0 : v >= 100 ? 0 : v >= 10 ? 1 : 2;
  return `${v.toFixed(digits)} ${units[i]}`;
}
