import { cn } from "../lib/utils.js";

export function Label({ className, ...props }) {
  return (
    <label
      className={cn("flex flex-col gap-1.5 text-sm font-medium text-foreground", className)}
      {...props}
    />
  );
}
