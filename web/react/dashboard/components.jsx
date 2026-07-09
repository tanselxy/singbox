import { useEffect } from "react";

export function Badge({ active, children }) {
  return <span className={`badge ${active ? "ok" : "down"}`}>{children}</span>;
}

export function PageHead({ meta, action }) {
  return (
    <div className="page-head">
      <div>
        <p className="eyebrow">{meta.eyebrow}</p>
        <h1>{meta.title}</h1>
      </div>
      {action}
    </div>
  );
}

export function Metric({ label, value, alert }) {
  return (
    <div className="metric-card">
      <div className="muted">{label}</div>
      <strong className={alert ? "alert" : ""}>{value}</strong>
    </div>
  );
}

export function ModalFrame({ open, onCancel, eyebrow, title, titleId, children, footer }) {
  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event) => {
      if (event.key === "Escape") onCancel();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, onCancel]);

  if (!open) return null;

  return (
    <div className="modal-layer" role="presentation" onMouseDown={onCancel}>
      <section
        className="modal-card"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <p className="eyebrow">{eyebrow}</p>
            <h3 id={titleId}>{title}</h3>
          </div>
          <button className="modal-close" aria-label="关闭" onClick={onCancel}>×</button>
        </div>
        <div className="modal-body form-grid">{children}</div>
        <div className="modal-foot">{footer}</div>
      </section>
    </div>
  );
}
