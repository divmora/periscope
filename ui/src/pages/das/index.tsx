import { useState, useEffect } from "react";
import { useParams, Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { DASAPI, DASRecord } from "../../api/das";
import { SourcesAPI } from "../../api/sources";
import { 
  Shield, Database, Search, RefreshCw, Clock, 
  Terminal, AlertTriangle, Copy, Check, 
  Cpu, Flame, Server, ExternalLink
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";

export default function DASPage() {
  const { projectId } = useParams();
  const [engine, setEngine] = useState<"direct" | "athena">("direct");
  const [selectedSourceIid, setSelectedSourceIid] = useState<string>("");
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedUser, setSelectedUser] = useState("");
  const [selectedCommand, setSelectedCommand] = useState("");
  const [selectedStatus, setSelectedStatus] = useState("");
  const [selectedRecord, setSelectedRecord] = useState<DASRecord | null>(null);
  const [copied, setCopied] = useState(false);

  // Fetch DAS Sources
  const { data: sourcesData, isLoading: isLoadingSources } = useQuery({
    queryKey: ['sources', projectId],
    queryFn: () => SourcesAPI.listSources(projectId as string),
    enabled: !!projectId,
  });

  const dasSources = (sourcesData?.data || []).filter(s => s.source_type === "aurora_das");

  useEffect(() => {
    if (!selectedSourceIid && dasSources.length > 0) {
      setSelectedSourceIid(dasSources[0].iid);
    }
  }, [dasSources, selectedSourceIid]);

  // Fetch DAS Logs
  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ['das_activity', projectId, selectedSourceIid, engine, searchTerm, selectedUser, selectedCommand, selectedStatus],
    queryFn: () => DASAPI.getActivity(projectId as string, {
      engine,
      source_iid: selectedSourceIid || undefined,
      search: searchTerm,
      user: selectedUser,
      command: selectedCommand,
      status: selectedStatus,
    }),
    enabled: !!projectId,
  });

  // Fetch DAS Summary Stats
  const { data: statsData } = useQuery({
    queryKey: ['das_stats', projectId, selectedSourceIid],
    queryFn: () => DASAPI.getStats(projectId as string, selectedSourceIid || undefined),
    enabled: !!projectId,
  });

  const records = data?.data || [];
  const stats = statsData?.data;

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const getCommandBadge = (command: string) => {
    switch (command.toUpperCase()) {
      case "SELECT":
        return <span className="px-2 py-0.5 rounded text-[11px] font-mono font-medium bg-blue-500/10 text-blue-400 border border-blue-500/20">SELECT</span>;
      case "UPDATE":
        return <span className="px-2 py-0.5 rounded text-[11px] font-mono font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20">UPDATE</span>;
      case "INSERT":
        return <span className="px-2 py-0.5 rounded text-[11px] font-mono font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">INSERT</span>;
      case "DELETE":
        return <span className="px-2 py-0.5 rounded text-[11px] font-mono font-medium bg-red-500/10 text-red-400 border border-red-500/20">DELETE</span>;
      case "ALTER":
      case "CREATE":
      case "DROP":
        return <span className="px-2 py-0.5 rounded text-[11px] font-mono font-medium bg-purple-500/10 text-purple-400 border border-purple-500/20">{command}</span>;
      default:
        return <span className="px-2 py-0.5 rounded text-[11px] font-mono font-medium bg-zinc-500/10 text-zinc-400 border border-zinc-500/20">{command}</span>;
    }
  };

  return (
    <div className="flex-1 space-y-6 p-8 pt-6">
      {/* Top Header & Engine Mode Switcher */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight text-foreground flex items-center gap-2.5">
              <Shield className="h-6 w-6 text-primary" />
              Aurora Database Activity Streams
            </h1>
          </div>
          <p className="text-sm text-muted-foreground mt-1">
            Real-time audit trails, distributed SQL execution logs, and compliance telemetry from Amazon Aurora.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          {/* Multi-Source Switcher */}
          {dasSources.length > 0 && (
            <div className="flex items-center gap-2 bg-card border border-border px-3 py-1.5 rounded-lg text-xs shadow-sm">
              <Database className="h-3.5 w-3.5 text-primary" />
              <span className="text-muted-foreground font-medium">Source:</span>
              <select
                value={selectedSourceIid}
                onChange={(e) => setSelectedSourceIid(e.target.value)}
                className="bg-transparent border-none text-foreground font-medium focus:outline-none cursor-pointer"
              >
                {dasSources.map((s) => (
                  <option key={s.iid} value={s.iid} className="bg-popover text-foreground">
                    {s.name}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Engine Switcher */}
          <div className="flex items-center bg-card border border-border p-1 rounded-lg">
            <button
              onClick={() => setEngine("direct")}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-all ${
                engine === "direct"
                  ? "bg-primary text-primary-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <Cpu className="h-3.5 w-3.5" />
              Direct S3 (DuckDB)
              <span className="text-[10px] bg-emerald-500/20 text-emerald-300 px-1.5 py-0.2 rounded-full border border-emerald-500/30">
                Fast &amp; Free
              </span>
            </button>
            <button
              onClick={() => setEngine("athena")}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-all ${
                engine === "athena"
                  ? "bg-primary text-primary-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <Server className="h-3.5 w-3.5" />
              Amazon Athena
              <span className="text-[10px] bg-blue-500/20 text-blue-300 px-1.5 py-0.2 rounded-full border border-blue-500/30">
                Distributed
              </span>
            </button>
          </div>
        </div>
      </div>

      {/* No DAS Sources Warning */}
      {dasSources.length === 0 && !isLoadingSources && (
        <div className="rounded-xl border border-dashed border-border bg-card/50 p-6 text-center">
          <Database className="h-10 w-10 text-muted-foreground mx-auto mb-3 opacity-60" />
          <h3 className="text-base font-semibold text-foreground">No Aurora DAS Sources Configured</h3>
          <p className="text-sm text-muted-foreground max-w-md mx-auto mt-1 mb-4">
            Connect an Amazon Aurora Database Activity Stream via Direct S3 or Athena to start analyzing real-time SQL execution audits.
          </p>
          <Link to={`/projects/${projectId}/settings`}>
            <Button size="sm" variant="outline" className="gap-2">
              <ExternalLink className="h-4 w-4" />
              Add Data Source in Settings
            </Button>
          </Link>
        </div>
      )}

      {/* KPI Stats Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
          <div className="flex items-center justify-between text-muted-foreground text-xs font-medium mb-1">
            <span>Total Logged Events</span>
            <Database className="h-4 w-4 text-primary" />
          </div>
          <div className="text-2xl font-bold text-foreground">
            {stats ? stats.total_queries.toLocaleString() : "..."}
          </div>
          <p className="text-[11px] text-muted-foreground mt-1">Hive-partitioned Parquet storage</p>
        </div>

        <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
          <div className="flex items-center justify-between text-muted-foreground text-xs font-medium mb-1">
            <span>Avg Query Duration</span>
            <Clock className="h-4 w-4 text-emerald-400" />
          </div>
          <div className="text-2xl font-bold text-foreground">
            {stats ? `${stats.avg_duration_ms.toFixed(2)}ms` : "..."}
          </div>
          <p className="text-[11px] text-muted-foreground mt-1">Database execution latency</p>
        </div>

        <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
          <div className="flex items-center justify-between text-muted-foreground text-xs font-medium mb-1">
            <span>Statement Failures</span>
            <AlertTriangle className="h-4 w-4 text-red-400" />
          </div>
          <div className="text-2xl font-bold text-foreground">
            {stats ? stats.total_errors : "0"}
          </div>
          <p className="text-[11px] text-muted-foreground mt-1">Timeouts &amp; authorization errors</p>
        </div>

        <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
          <div className="flex items-center justify-between text-muted-foreground text-xs font-medium mb-1">
            <span>Current OLAP Driver</span>
            <Flame className="h-4 w-4 text-amber-400" />
          </div>
          <div className="text-sm font-bold text-foreground mt-1 flex items-center gap-1.5">
            {engine === "direct" ? (
              <span className="text-emerald-400 font-mono">Embedded DuckDB</span>
            ) : (
              <span className="text-blue-400 font-mono">AWS Athena API</span>
            )}
          </div>
          <p className="text-[11px] text-muted-foreground mt-1">
            {engine === "direct" ? "Zero-cost Parquet scanning" : "AWS Glue Federated Query"}
          </p>
        </div>
      </div>

      {/* Filter and Search Bar */}
      <div className="flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3 bg-card/60 p-3 rounded-xl border border-border">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search SQL query, table, or user..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="pl-9 bg-background/50 border-border text-sm"
          />
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {/* Command Filter */}
          <select
            value={selectedCommand}
            onChange={(e) => setSelectedCommand(e.target.value)}
            className="bg-background/80 border border-border rounded-md px-3 py-1.5 text-xs text-foreground focus:outline-none focus:border-primary"
          >
            <option value="">All Commands</option>
            <option value="SELECT">SELECT</option>
            <option value="UPDATE">UPDATE</option>
            <option value="INSERT">INSERT</option>
            <option value="DELETE">DELETE</option>
            <option value="ALTER">ALTER / DDL</option>
          </select>

          {/* Status Filter */}
          <select
            value={selectedStatus}
            onChange={(e) => setSelectedStatus(e.target.value)}
            className="bg-background/80 border border-border rounded-md px-3 py-1.5 text-xs text-foreground focus:outline-none focus:border-primary"
          >
            <option value="">All Statuses</option>
            <option value="success">Success</option>
            <option value="failure">Error / Failure</option>
          </select>

          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="text-xs h-8 border-border"
          >
            <RefreshCw className={`h-3.5 w-3.5 mr-1.5 ${isFetching ? "animate-spin text-primary" : ""}`} />
            Refresh
          </Button>
        </div>
      </div>

      {/* Activity Table */}
      <div className="rounded-xl border border-border bg-card overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse text-xs">
            <thead>
              <tr className="border-b border-border bg-muted/30 text-muted-foreground font-medium">
                <th className="py-3 px-4">Timestamp</th>
                <th className="py-3 px-4">Database</th>
                <th className="py-3 px-4">DB User</th>
                <th className="py-3 px-4">Client IP</th>
                <th className="py-3 px-4">Command</th>
                <th className="py-3 px-4">SQL Statement</th>
                <th className="py-3 px-4">Duration</th>
                <th className="py-3 px-4">Status</th>
                <th className="py-3 px-4 text-right">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border/60">
              {isLoading ? (
                <tr>
                  <td colSpan={9} className="py-12 text-center text-muted-foreground">
                    <RefreshCw className="h-6 w-6 animate-spin text-primary mx-auto mb-2" />
                    Executing query via {engine === "direct" ? "DuckDB" : "Amazon Athena"}...
                  </td>
                </tr>
              ) : records.length === 0 ? (
                <tr>
                  <td colSpan={9} className="py-12 text-center text-muted-foreground">
                    <Shield className="h-8 w-8 text-muted-foreground/60 mx-auto mb-2" />
                    No Database Activity Stream records found matching filters.
                  </td>
                </tr>
              ) : (
                records.map((r: DASRecord) => (
                  <tr
                    key={r.id}
                    onClick={() => setSelectedRecord(r)}
                    className="hover:bg-muted/30 cursor-pointer transition-colors"
                  >
                    <td className="py-3 px-4 font-mono text-[11px] text-muted-foreground whitespace-nowrap">
                      {new Date(r.event_time).toLocaleTimeString()}
                    </td>
                    <td className="py-3 px-4 font-medium text-foreground whitespace-nowrap">
                      {r.database_name}
                    </td>
                    <td className="py-3 px-4 font-mono text-zinc-300">
                      {r.db_user}
                    </td>
                    <td className="py-3 px-4 font-mono text-[11px] text-muted-foreground">
                      {r.client_ip}
                    </td>
                    <td className="py-3 px-4">
                      {getCommandBadge(r.command)}
                    </td>
                    <td className="py-3 px-4 font-mono text-[11px] text-zinc-300 max-w-xs truncate" title={r.command_text}>
                      {r.command_text}
                    </td>
                    <td className="py-3 px-4 font-mono text-[11px] text-muted-foreground whitespace-nowrap">
                      {r.execution_time_ms ? `${r.execution_time_ms.toFixed(2)}ms` : "—"}
                    </td>
                    <td className="py-3 px-4">
                      {r.status === "failure" ? (
                        <span className="flex items-center gap-1 text-[11px] text-red-400 bg-red-500/10 px-2 py-0.5 rounded-full border border-red-500/20">
                          Error
                        </span>
                      ) : (
                        <span className="flex items-center gap-1 text-[11px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-full border border-emerald-500/20">
                          OK
                        </span>
                      )}
                    </td>
                    <td className="py-3 px-4 text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 text-xs text-muted-foreground hover:text-foreground"
                        onClick={(e) => {
                          e.stopPropagation();
                          setSelectedRecord(r);
                        }}
                      >
                        Inspect
                      </Button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Record Inspection Modal */}
      <Dialog open={!!selectedRecord} onOpenChange={(open) => !open && setSelectedRecord(null)}>
        <DialogContent className="max-w-2xl bg-card border-border">
          {selectedRecord && (
            <>
              <DialogHeader>
                <div className="flex items-center justify-between mr-6">
                  <DialogTitle className="flex items-center gap-2 text-base">
                    <Terminal className="h-5 w-5 text-primary" />
                    SQL Execution Details
                  </DialogTitle>
                  {getCommandBadge(selectedRecord.command)}
                </div>
                <DialogDescription>
                  Executed on <span className="font-mono text-foreground">{selectedRecord.database_name}</span> at{" "}
                  {new Date(selectedRecord.event_time).toLocaleString()}
                </DialogDescription>
              </DialogHeader>

              <div className="space-y-4 pt-2">
                {/* Meta details */}
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 bg-muted/40 p-3 rounded-lg border border-border text-xs">
                  <div>
                    <span className="text-muted-foreground block text-[11px]">Database User</span>
                    <span className="font-mono text-foreground font-medium">{selectedRecord.db_user}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground block text-[11px]">Client IP</span>
                    <span className="font-mono text-foreground font-medium">{selectedRecord.client_ip}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground block text-[11px]">Client Application</span>
                    <span className="text-foreground truncate block font-medium">{selectedRecord.client_app || "Unknown"}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground block text-[11px]">Execution Time</span>
                    <span className="font-mono text-foreground font-medium">{selectedRecord.execution_time_ms}ms</span>
                  </div>
                </div>

                {/* Error Banner if Failed */}
                {selectedRecord.error_message && (
                  <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-xs text-red-400 flex items-start gap-2">
                    <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5" />
                    <div>
                      <span className="font-semibold block">Execution Error</span>
                      <p className="font-mono mt-0.5">{selectedRecord.error_message}</p>
                    </div>
                  </div>
                )}

                {/* SQL Statement Codebox */}
                <div className="space-y-1.5">
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span>Full SQL Statement</span>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleCopy(selectedRecord.command_text)}
                      className="h-6 px-2 text-[11px] text-muted-foreground hover:text-foreground"
                    >
                      {copied ? (
                        <>
                          <Check className="h-3 w-3 mr-1 text-emerald-400" />
                          Copied
                        </>
                      ) : (
                        <>
                          <Copy className="h-3 w-3 mr-1" />
                          Copy SQL
                        </>
                      )}
                    </Button>
                  </div>
                  <pre className="p-4 rounded-lg bg-background border border-border font-mono text-xs text-zinc-200 overflow-x-auto whitespace-pre-wrap leading-relaxed max-h-64">
                    {selectedRecord.command_text}
                  </pre>
                </div>
              </div>
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
