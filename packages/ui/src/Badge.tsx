import type { HTMLAttributes, ReactNode } from "react";

export type BadgeVariant =
  "iris" | "sky" | "teal" | "amber" | "rose" | "neutral";
export type BadgeSize = "sm" | "md";

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  children: ReactNode;
  variant?: BadgeVariant;
  size?: BadgeSize;
}

const variantStyles: Record<
  BadgeVariant,
  { backgroundColor: string; color: string }
> = {
  iris: {
    backgroundColor: "#e9e7ff",
    color: "#4338ca",
  },
  sky: {
    backgroundColor: "#e0f2fe",
    color: "#0369a1",
  },
  teal: {
    backgroundColor: "#ccfbf1",
    color: "#0f766e",
  },
  amber: {
    backgroundColor: "#fef3c7",
    color: "#b45309",
  },
  rose: {
    backgroundColor: "#ffe4e6",
    color: "#be123c",
  },
  neutral: {
    backgroundColor: "#f3f4f6",
    color: "#374151",
  },
};

const sizeStyles: Record<BadgeSize, { fontSize: string; padding: string }> = {
  sm: {
    fontSize: "0.75rem",
    padding: "0.2rem 0.6rem",
  },
  md: {
    fontSize: "0.875rem",
    padding: "0.35rem 0.8rem",
  },
};

export function Badge({
  children,
  variant = "neutral",
  size = "md",
  style,
  ...props
}: BadgeProps) {
  return (
    <span
      role="status"
      data-variant={variant}
      style={{
        alignItems: "center",
        backgroundColor: variantStyles[variant].backgroundColor,
        borderRadius: "9999px",
        color: variantStyles[variant].color,
        display: "inline-flex",
        fontSize: sizeStyles[size].fontSize,
        fontWeight: 600,
        lineHeight: 1.2,
        padding: sizeStyles[size].padding,
        whiteSpace: "nowrap",
        ...style,
      }}
      {...props}
    >
      {children}
    </span>
  );
}
