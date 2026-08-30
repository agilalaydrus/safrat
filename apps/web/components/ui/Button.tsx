import React from "react";

type Variant = "gold" | "emerald" | "outline" | "ghost" | "danger";
type Size = "sm" | "md" | "lg";

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  children: React.ReactNode;
}

export function Button({ variant = "gold", size = "md", className, children, ...props }: ButtonProps) {
  const classes = ["tw-btn", `tw-btn--${variant}`, `tw-btn--${size}`, className]
    .filter(Boolean)
    .join(" ");

  return <button className={classes} {...props}>{children}</button>;
}
