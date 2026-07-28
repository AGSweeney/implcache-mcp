type Variant = "full" | "mark";

export default function BrandLogo({
  variant = "full",
  className = "",
}: {
  variant?: Variant;
  className?: string;
}) {
  const base = import.meta.env.BASE_URL;
  if (variant === "mark") {
    return (
      <img
        className={`brand-logo brand-logo-mark ${className}`.trim()}
        src={`${base}favicon.png`}
        alt="ImplCache"
        width={32}
        height={32}
      />
    );
  }
  return (
    <img
      className={`brand-logo brand-logo-full ${className}`.trim()}
      src={`${base}logo-implcache-on-dark.png`}
      alt="ImplCache"
      width={220}
      height={40}
    />
  );
}
