import { NavLink } from "react-router";
import { routePath, VIEW_META } from "./constants.js";

export function Shell({ children }) {
  return (
    <main className="console-shell">
      <aside className="console-sidebar">
        {Object.entries(VIEW_META).map(([key, item]) => (
          <NavLink
            key={key}
            className={({ isActive }) => `side-link ${isActive ? "active" : ""}`}
            end={key === "overview"}
            to={routePath(key)}
          >
            {item.label}
          </NavLink>
        ))}
      </aside>
      <section className="console-content">{children}</section>
    </main>
  );
}
