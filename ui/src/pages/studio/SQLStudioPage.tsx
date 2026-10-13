import { useState } from "react";
import { useParams } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import { QueryAPI, QueryExecutionResult } from "../../api/query";
import { 
  Play, Download, Clock, Database, Terminal, 
  Sparkles, AlertCircle, Layers, CheckCircle2, Copy 
} from "lucide-react";
import { Button } from "@/components/ui/button";

const SAMPLE_QUERIES = [
  {
    title: "DuckDB Engine Status",
    sql: "SELECT \n  version() AS duckdb_version, \n  current_timestamp AS executed_at, \n  'OLAP Engine Online' AS status;",
  },
  {
    title: "Parquet Scan Simulation",
    sql: "SELECT \n  'HTTP 200' AS status_code,\n  count(*) AS total_hits,\n  avg(duration_ms) AS avg_latency_ms,\n  approx_quantile(duration_ms, 0.99) AS p99_latency_ms\nFROM (VALUES \n  (200, 45.2), (200, 32.1), (200, 110.5), (500, 450.0), (200, 21.0)\n) AS t(status_code, duration_ms)\nGROUP BY status_code;",
  },
  {
    title: "Database Activity Stream Breakdown",
    sql: "SELECT \n  command,\n  count(*) as executions,\n  round(avg(duration_ms), 2) as avg_duration_ms\nFROM (VALUES \n  ('SELECT', 1240, 2.4),\n  ('UPDATE', 450, 8.1),\n  ('INSERT', 310, 5.7),\n  ('DELETE', 22, 14.3)\n) as das(command, count, duration_ms)\nGROUP BY command, duration_ms\nORDER BY executions DESC;",
  },
  {
    title: "DuckDB System Information",
    sql: "PRAGMA version;\nSELECT current_setting('threads') AS active_threads, current_setting('memory_limit') AS memory_limit;",
  },
];

export default function SQLStudioPage() {
  const { projectId } = useParams();
  const [sql, setSql] = useState(SAMPLE_QUERIES[0].sql);
  const [result, setResult] = useState<QueryExecutionResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const executeMutation = useMutation({
    mutationFn: (queryText: string) => QueryAPI.executeSQL(projectId as string, queryText),
    onSuccess: (res) => {
      if (res.success && res.data) {
        setResult(res.data);
        setError(null);
      } else {
        setError(res.error?.message || "Query failed");
        setResult(null);
      }
    },
    onError: (err: any) => {
      setError(err?.message || "Failed to execute query");
      setResult(null);
    },
  });

  const handleRun = () => {
    if (!sql.trim()) return;
    executeMutation.mutate(sql);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
      e.preventDefault();
      handleRun();
    }
  };

  const downloadCSV = () => {
    if (!result || !result.columns || !result.rows) return;
    const header = result.columns.join(",");
    const rows = result.rows.map(r => r.map(c => `"${String(c).replace(/"/g, '""')}"`).join(","));
    const csvContent = "data:text/csv;charset=utf-8," + [header, ...rows].join("\n");
    const encodedUri = encodeURI(csvContent);
    const link = document.createElement("a");
    link.setAttribute("href", encodedUri);
    link.setAttribute("download", `periscope_query_${Date.now()}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  return (
    <div className="flex flex-col h-full space-y-4 p-8 pt-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground flex items-center gap-3">
            <Terminal className="h-6 w-6 text-primary" />
            DuckDB SQL Studio
            <span className="px-2.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-xs font-semibold">
              Embedded OLAP
            </span>
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Execute zero-copy analytical SQL queries directly over cold Parquet files in S3 and local storage.
          </p>
        </div>

        {/* Template Selector */}
        <div className="flex items-center gap-2">
          <select
            onChange={(e) => {
              const selected = SAMPLE_QUERIES.find(q => q.title === e.target.value);
              if (selected) setSql(selected.sql);
            }}
            className="bg-card border border-border rounded-md px-3 py-1.5 text-xs text-foreground focus:outline-none focus:border-primary"
          >
            <option value="">Query Templates...</option>
            {SAMPLE_QUERIES.map(q => (
              <option key={q.title} value={q.title}>{q.title}</option>
            ))}
          </select>
          <Button
            onClick={handleRun}
            disabled={executeMutation.isPending}
            className="bg-primary text-primary-foreground hover:bg-primary/90 flex items-center gap-2"
          >
            <Play className={`h-4 w-4 ${executeMutation.isPending ? "animate-spin" : ""}`} />
            Run Query <span className="text-[10px] opacity-75 hidden sm:inline">(Ctrl+Enter)</span>
          </Button>
        </div>
      </div>

      {/* SQL Editor Area */}
      <div className="rounded-xl border border-border bg-[#09090b] overflow-hidden shadow-sm flex flex-col">
        <div className="px-4 py-2 bg-muted/30 border-b border-border flex items-center justify-between text-xs text-muted-foreground font-mono">
          <div className="flex items-center gap-2">
            <Terminal className="h-3.5 w-3.5 text-primary" />
            <span>SQL Workbench</span>
          </div>
          <button
            onClick={() => navigator.clipboard.writeText(sql)}
            className="hover:text-foreground flex items-center gap-1 transition-colors"
            title="Copy SQL"
          >
            <Copy className="h-3 w-3" />
            Copy
          </button>
        </div>
        <textarea
          value={sql}
          onChange={(e) => setSql(e.target.value)}
          onKeyDown={handleKeyDown}
          rows={6}
          placeholder="SELECT * FROM read_parquet('s3://...') LIMIT 100;"
          className="w-full bg-transparent p-4 font-mono text-sm text-foreground focus:outline-none resize-y selection:bg-primary/30"
          spellCheck={false}
        />
      </div>

      {/* Execution Results */}
      {error && (
        <div className="rounded-xl border border-destructive/30 bg-destructive/10 p-4 text-destructive flex items-start gap-3">
          <AlertCircle className="h-5 w-5 shrink-0 mt-0.5" />
          <div>
            <h4 className="text-sm font-semibold">Query Error</h4>
            <p className="text-xs font-mono mt-1 opacity-90">{error}</p>
          </div>
        </div>
      )}

      {result && (
        <div className="flex-1 flex flex-col rounded-xl border border-border bg-card overflow-hidden min-h-[300px]">
          {/* Results Bar */}
          <div className="px-4 py-2.5 bg-muted/40 border-b border-border flex items-center justify-between text-xs">
            <div className="flex items-center gap-4 text-muted-foreground">
              <span className="flex items-center gap-1 text-emerald-400 font-medium">
                <CheckCircle2 className="h-3.5 w-3.5" />
                Query Succeeded
              </span>
              <span className="flex items-center gap-1">
                <Clock className="h-3.5 w-3.5" />
                {result.duration_ms.toFixed(2)} ms
              </span>
              <span>
                <strong className="text-foreground">{result.row_count}</strong> {result.row_count === 1 ? "row" : "rows"}
              </span>
              {result.truncated && (
                <span className="text-amber-400 text-[11px]">(capped at 1000 rows)</span>
              )}
            </div>

            <Button
              variant="outline"
              size="sm"
              onClick={downloadCSV}
              className="h-7 text-xs flex items-center gap-1.5"
            >
              <Download className="h-3.5 w-3.5" />
              Export CSV
            </Button>
          </div>

          {/* Result Grid */}
          <div className="flex-1 overflow-auto">
            <table className="w-full text-left text-sm font-mono border-collapse">
              <thead className="bg-muted/60 text-muted-foreground text-xs uppercase sticky top-0 border-b border-border">
                <tr>
                  {result.columns.map((col, idx) => (
                    <th key={idx} className="px-4 py-2.5 font-semibold text-foreground whitespace-nowrap">
                      {col}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-border/60">
                {result.rows.length === 0 ? (
                  <tr>
                    <td colSpan={result.columns.length} className="px-4 py-8 text-center text-muted-foreground text-xs">
                      Query returned 0 rows.
                    </td>
                  </tr>
                ) : (
                  result.rows.map((row, rIdx) => (
                    <tr key={rIdx} className="hover:bg-muted/20 transition-colors">
                      {row.map((cell, cIdx) => (
                        <td key={cIdx} className="px-4 py-2 text-xs text-foreground whitespace-nowrap">
                          {cell === null ? (
                            <span className="text-muted-foreground/60 italic">NULL</span>
                          ) : typeof cell === "object" ? (
                            JSON.stringify(cell)
                          ) : (
                            String(cell)
                          )}
                        </td>
                      ))}
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
