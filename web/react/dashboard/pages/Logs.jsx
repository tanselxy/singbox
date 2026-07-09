import { useEffect, useState } from "react";
import { PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";

export function Logs({ prefix }) {
  const [entries, setEntries] = useState([]);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const pageSize = 50;

  async function loadLogs(nextPage) {
    setLoading(true);
    setError("");
    try {
      const res = await fetch(`${prefix}/api/logs?page=${nextPage}&size=${pageSize}`);
      const payload = await res.json();
      if (!payload.ok) {
        setError(payload.error || "日志加载失败");
        setEntries([]);
        setHasMore(false);
        return;
      }
      setEntries(payload.entries || []);
      setHasMore(Boolean(payload.has_more));
      setPage(payload.page || nextPage);
    } catch (err) {
      setError(err.message);
      setEntries([]);
      setHasMore(false);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadLogs(1);
  }, []);

  return (
    <>
      <PageHead meta={VIEW_META.logs} />
      <section className="card logs-table-panel">
        <div className="node-head logs-toolbar">
          <h3>最近日志</h3>
          <span className="muted small">第 {page} 页，每页 {pageSize} 行</span>
        </div>
        <div className="table-wrap">
          <table className="clients logs-table">
            <thead>
              <tr><th>时间</th><th>来源</th><th>内容</th></tr>
            </thead>
            <tbody>
              {loading && <tr><td colSpan="3" className="muted">正在加载日志。</td></tr>}
              {!loading && error && <tr><td colSpan="3" className="alert">{error}</td></tr>}
              {!loading && !error && entries.length === 0 && <tr><td colSpan="3" className="muted">暂无日志。</td></tr>}
              {!loading && !error && entries.map((entry, index) => (
                <tr key={`${entry.time}-${index}`}>
                  <td className="mono small log-time">{entry.time || "-"}</td>
                  <td className="mono small log-source">{entry.source || "-"}</td>
                  <td className="log-message">{entry.message || "-"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="pager">
          <button onClick={() => loadLogs(page - 1)} disabled={loading || page <= 1}>上一页</button>
          <span className="muted small">第 {page} 页</span>
          <button onClick={() => loadLogs(page + 1)} disabled={loading || !hasMore}>下一页</button>
        </div>
      </section>
    </>
  );
}
