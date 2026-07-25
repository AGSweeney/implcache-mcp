import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from "react";

type Variant = "primary" | "secondary" | "ghost" | "danger" | "icon";

const Button = forwardRef<
  HTMLButtonElement,
  ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; children?: ReactNode }
>(function Button({ variant = "secondary", children, className = "", ...rest }, ref) {
  return (
    <button ref={ref} type="button" className={`btn btn-${variant} ${className}`.trim()} {...rest}>
      {children}
    </button>
  );
});

export default Button;
