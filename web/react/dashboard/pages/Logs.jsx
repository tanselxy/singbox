import { useEffect, useState } from "react";
import { PageHead } from "../components.jsx";
import { VIEW_META } from "../constants.js";
import { Button } from "../../ui/button.jsx";
import { Card, CardContent, CardHeader, CardTitle } from "../../ui/card.jsx";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../ui/table.jsx";

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
      <Card>
        <CardHeader>
          <CardTitle>最近日志</CardTitle>
          <span className="text-sm text-muted-foreground">第 {page} 页，每页 {pageSize} 行</span>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>时间</TableHead>
                <TableHead>来源</TableHead>
                <TableHead>内容</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading && <TableRow><TableCell colSpan="3" className="text-muted-foreground">正在加载日志。</TableCell></TableRow>}
              {!loading && error && <TableRow><TableCell colSpan="3" className="text-destructive">{error}</TableCell></TableRow>}
              {!loading && !error && entries.length === 0 && <TableRow><TableCell colSpan="3" className="text-muted-foreground">暂无日志。</TableCell></TableRow>}
              {!loading && !error && entries.map((entry, index) => (
                <TableRow key={`${entry.time}-${index}`}>
                  <TableCell className="font-mono text-xs whitespace-nowrap text-muted-foreground">{entry.time || "-"}</TableCell>
                  <TableCell className="font-mono text-xs whitespace-nowrap text-muted-foreground">{entry.source || "-"}</TableCell>
                  <TableCell className="font-mono text-xs break-all">{entry.message || "-"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <div className="mt-4 flex items-center justify-center gap-4">
            <Button variant="outline" size="sm" onClick={() => loadLogs(page - 1)} disabled={loading || page <= 1}>上一页</Button>
            <span className="text-sm text-muted-foreground">第 {page} 页</span>
            <Button variant="outline" size="sm" onClick={() => loadLogs(page + 1)} disabled={loading || !hasMore}>下一页</Button>
          </div>
        </CardContent>
      </Card>
    </>
  );
}
