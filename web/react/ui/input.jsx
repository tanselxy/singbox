import { cn } from "../lib/utils.js";

const base =
  "flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50";

export function Input({ className, type = "text", ...props }) {
  return <input type={type} className={cn(base, className)} {...props} />;
}

export function Textarea({ className, ...props }) {
  return <textarea className={cn(base, "h-auto min-h-16 py-2", className)} {...props} />;
}

export function Select({ className, children, ...props }) {
  return (
    <select className={cn(base, "cursor-pointer", className)} {...props}>
      {children}
    </select>
  );
}
