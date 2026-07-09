import { NavLink } from "react-router";
import { routePath, VIEW_META } from "./constants.js";
import { cn } from "../lib/utils.js";

export function Shell({ children }) {
  return (
    <div className="mx-auto flex min-h-screen w-full max-w-7xl gap-6 p-4 md:p-6">
      <aside className="sticky top-6 hidden h-fit w-52 shrink-0 flex-col gap-1 md:flex">
        <div className="mb-4 px-3 py-2">
          <span className="text-base font-semibold tracking-tight">Sing-Box</span>
          <span className="ml-1 text-xs text-muted-foreground">面板</span>
        </div>
        {Object.entries(VIEW_META).map(([key, item]) => (
          <NavLink
            key={key}
            end={key === "overview"}
            to={routePath(key)}
            className={({ isActive }) =>
              cn(
                "rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
                isActive && "bg-accent text-accent-foreground",
              )
            }
          >
            {item.label}
          </NavLink>
        ))}
      </aside>
      <section className="min-w-0 flex-1">{children}</section>
    </div>
  );
}
