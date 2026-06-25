import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

type TableProps = ComponentProps<"table"> & { containerClassName?: string };

export function Table({ className, containerClassName, ...props }: TableProps) {
  return (
    <div className={cn("overflow-hidden rounded-xl border border-slate-200 bg-white shadow-xs", containerClassName)}>
      <div className="overflow-x-auto">
        <table data-slot="table" className={cn("min-w-full text-sm text-slate-700", className)} {...props} />
      </div>
    </div>
  );
}
export function TableHead({ className, ...props }: ComponentProps<"thead">) {
  return <thead data-slot="table-head" className={cn("bg-slate-50 text-left text-xs font-medium text-slate-500", className)} {...props} />;
}
export function TableBody({ className, ...props }: ComponentProps<"tbody">) {
  return <tbody data-slot="table-body" className={cn("divide-y divide-slate-100", className)} {...props} />;
}
export function TableRow({ className, ...props }: ComponentProps<"tr">) {
  return <tr data-slot="table-row" className={cn("transition-colors hover:bg-slate-50/80", className)} {...props} />;
}
export function TableHeaderCell({ className, ...props }: ComponentProps<"th">) {
  return <th data-slot="table-header-cell" className={cn("px-4 py-3 font-medium", className)} {...props} />;
}
export function TableCell({ className, ...props }: ComponentProps<"td">) {
  return <td data-slot="table-cell" className={cn("px-4 py-3 align-middle text-slate-700", className)} {...props} />;
}
