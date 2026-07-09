import { Badge as UiBadge } from "../ui/badge.jsx";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../ui/dialog.jsx";
import { cn } from "../lib/utils.js";

export function Badge({ active, children }) {
  return <UiBadge variant={active ? "success" : "secondary"}>{children}</UiBadge>;
}

export function PageHead({ meta, action }) {
  return (
    <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
      <div>
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">{meta.eyebrow}</p>
        <h1 className="mt-1 text-2xl font-semibold tracking-tight">{meta.title}</h1>
      </div>
      {action}
    </div>
  );
}

export function Metric({ label, value, alert }) {
  return (
    <div className="rounded-xl border bg-card p-5 shadow-sm">
      <div className="text-sm text-muted-foreground">{label}</div>
      <strong className={cn("mt-1 block text-2xl font-semibold tabular-nums", alert && "text-destructive")}>
        {value}
      </strong>
    </div>
  );
}

// ModalFrame keeps its original API but renders a shadcn/Radix dialog.
export function ModalFrame({ open, onCancel, eyebrow, title, children, footer, className }) {
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) onCancel();
      }}
    >
      <DialogContent className={className}>
        <DialogHeader>
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">{eyebrow}</p>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <div className="grid gap-4">{children}</div>
        <DialogFooter>{footer}</DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
