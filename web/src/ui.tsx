import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { clsx } from "clsx";
import { twMerge } from "tailwind-merge";
export const cn = (...v: (string | undefined | false)[]) => twMerge(clsx(v));
const variants = cva(
  "ff-button inline-flex items-center justify-center gap-2 rounded-xl text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-emerald-600",
  {
    variants: {
      variant: {
        default: "bg-emerald-800 text-white shadow-sm hover:bg-emerald-900",
        outline:
          "border border-slate-200 bg-white text-slate-700 hover:border-emerald-300 hover:bg-emerald-50",
        ghost: "text-slate-600 hover:bg-emerald-50 hover:text-emerald-800",
        danger: "bg-rose-600 text-white hover:bg-rose-700",
      },
      size: {
        default: "min-h-11 px-4 py-2.5",
        sm: "min-h-9 px-3 py-1.5",
        lg: "min-h-12 px-6 py-3",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  },
);
export function Button({
  asChild = false,
  variant,
  size,
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof variants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot : "button";
  return (
    <Comp className={cn(variants({ variant, size }), className)} {...props} />
  );
}
export function Card({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section
      className={cn(
        "ff-card min-w-0 rounded-2xl border border-slate-200 bg-white p-5",
        className,
      )}
    >
      {children}
    </section>
  );
}
export function Field({
  label,
  className,
  ...props
}: { label: string } & React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="ff-field block text-sm font-medium text-slate-700">
      {label}
      <input
        className={cn(
          "mt-2 min-w-0 w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 outline-none transition focus:border-emerald-600 focus:ring-3 focus:ring-emerald-100",
          className,
        )}
        {...props}
      />
    </label>
  );
}
export function Notice({
  children,
  tone = "info",
}: {
  children: React.ReactNode;
  tone?: "info" | "error" | "success";
}) {
  return (
    <div
      role={tone === "error" ? "alert" : undefined}
      className={cn(
        "rounded-xl border p-3 text-sm leading-6",
        tone === "error"
          ? "border-rose-100 bg-rose-50 text-rose-800"
          : tone === "success"
            ? "border-emerald-100 bg-emerald-50 text-emerald-800"
            : "border-sky-100 bg-sky-50 text-sky-800",
      )}
    >
      {children}
    </div>
  );
}
