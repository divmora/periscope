import { useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { TelescopeAPI } from "../../api/telescope";
import { 
  FileText, 
  Search, 
  ChevronDown, 
  RotateCcw, 
  Filter, 
  Eye, 
  Loader2, 
  Archive, 
  ChevronLeft, 
  ChevronRight, 
  ChevronsLeft, 
  ChevronsRight 
} from "lucide-react";

export default function LogsPage() {
  const { projectId } = useParams();
  const [searchTerm, setSearchTerm] = useState("");
  const [page, setPage] = useState(1);

  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ['logs', projectId, page],
    queryFn: () => TelescopeAPI.getLogs(projectId as string, page),
    enabled: !!projectId,
  });

  const logs = data?.data || [];
  const meta = data?.meta;

  const getLevelBadge = (level: string) => {
    const lvl = (level || "info").toLowerCase();
    switch (lvl) {
      case "error":
      case "critical":
      case "alert":
      case "emergency":
        return <span className="px-2 py-0.5 rounded-full bg-[#18181b] border border-border text-xs font-medium text-red-500">{level.toUpperCase()}</span>;
      case "warning":
        return <span className="px-2 py-0.5 rounded-full bg-[#18181b] border border-border text-xs font-medium text-amber-500">{level.toUpperCase()}</span>;
      case "notice":
      case "info":
        return <span className="px-2 py-0.5 rounded-full bg-[#18181b] border border-border text-xs font-medium text-blue-500">{level.toUpperCase()}</span>;
      case "debug":
      default:
        return <span className="px-2 py-0.5 rounded-full bg-[#18181b] border border-border text-xs font-medium text-muted-foreground">{level.toUpperCase()}</span>;
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* Page Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <div className="flex items-center gap-3 mb-1">
            <div className="w-8 h-8 rounded-md bg-[#18181b] border border-border flex items-center justify-center">
              <FileText className="h-4 w-4 text-blue-500" />
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-foreground flex items-center gap-3">
              Logs
              <span className="px-2.5 py-0.5 rounded-full bg-[#18181b] border border-border text-xs font-medium text-muted-foreground">
                {meta?.total || 0} recorded
              </span>
            </h1>
          </div>
          <p className="text-sm text-muted-foreground">
            Monitor and review application logs emitted by your Laravel application.
          </p>
        </div>
        
        <button 
          onClick={() => refetch()}
          disabled={isFetching}
          className="flex items-center gap-2 bg-foreground hover:bg-foreground/90 text-background px-4 py-2 rounded-md text-sm font-medium transition-colors disabled:opacity-50"
        >
          <RotateCcw className={`h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {/* Filter Bar */}
      <div className="flex items-center gap-2 mb-4">
        <button className="flex items-center gap-2 bg-[#18181b] border border-border px-3 py-1.5 rounded-md text-sm text-foreground hover:bg-[#27272a] transition-colors">
          <Filter className="h-4 w-4" />
          Level
          <ChevronDown className="h-3 w-3 ml-1 text-muted-foreground" />
        </button>
        
        <div className="flex-1 relative flex items-center">
          <Search className="h-4 w-4 absolute left-3 text-muted-foreground" />
          <input 
            type="text" 
            placeholder="Search log messages..."
            className="w-full bg-[#09090b] border border-border rounded-md pl-10 pr-10 py-1.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:border-primary transition-colors"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>

        <button className="flex items-center gap-2 bg-[#18181b] border border-border px-3 py-1.5 rounded-md text-sm text-foreground hover:bg-[#27272a] transition-colors">
          <Filter className="h-4 w-4" />
          Time Range
          <ChevronDown className="h-3 w-3 ml-1 text-muted-foreground" />
        </button>
      </div>

      {/* Data Table */}
      <div className="border border-border rounded-md bg-[#09090b] flex flex-col flex-1 overflow-hidden">
        <div className="overflow-x-auto flex-1">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-muted-foreground uppercase bg-[#18181b]/50 border-b border-border">
              <tr>
                <th className="px-4 py-3 font-medium flex items-center gap-1">ID <ChevronDown className="h-3 w-3" /></th>
                <th className="px-4 py-3 font-medium">Level</th>
                <th className="px-4 py-3 font-medium">Message</th>
                <th className="px-4 py-3 font-medium">Time</th>
                <th className="px-4 py-3 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                <tr>
                  <td colSpan={5} className="px-4 py-12 text-center text-muted-foreground">
                    <Loader2 className="h-6 w-6 animate-spin mx-auto mb-2 text-primary" />
                    Loading logs...
                  </td>
                </tr>
              ) : logs.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-4 py-12 text-center text-muted-foreground">
                    <Archive className="h-8 w-8 mx-auto mb-3 text-border" />
                    No logs found matching your filters.
                  </td>
                </tr>
              ) : (
                logs.map((log: any) => (
                  <tr key={log.id} className="border-b border-border hover:bg-[#18181b] transition-colors group">
                    <td className="px-4 py-3 font-mono text-xs">
                      <span className="bg-[#27272a] px-1.5 py-0.5 rounded text-muted-foreground">LOG-{String(log.id).padStart(4, '0')}</span>
                    </td>
                    <td className="px-4 py-3">
                      {getLevelBadge(log.level)}
                    </td>
                    <td className="px-4 py-3 text-foreground truncate max-w-[400px]" title={log.message}>
                      {log.message}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {log.time}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <button className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium text-foreground bg-[#18181b] hover:bg-[#27272a] border border-border rounded transition-colors">
                        <Eye className="h-3 w-3 text-primary" />
                        View Details
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination Footer */}
        <div className="flex items-center justify-between px-4 py-3 border-t border-border bg-[#09090b]">
          <div className="text-xs text-muted-foreground">
            Showing <span className="font-medium text-foreground">{logs.length}</span> of <span className="font-medium text-foreground">{meta?.total || 0}</span> logs
          </div>
          <div className="flex items-center gap-4 text-xs">
            <div className="flex items-center gap-2">
              <span className="text-muted-foreground">Rows per page:</span>
              <button className="flex items-center gap-1 bg-[#18181b] border border-border px-2 py-1 rounded hover:bg-[#27272a] transition-colors">
                10 <ChevronDown className="h-3 w-3 text-muted-foreground" />
              </button>
            </div>
            <div className="flex items-center gap-1">
              <button className="p-1 rounded text-muted-foreground hover:bg-[#27272a] hover:text-foreground disabled:opacity-50" disabled>
                <ChevronsLeft className="h-4 w-4" />
              </button>
              <button className="p-1 rounded text-muted-foreground hover:bg-[#27272a] hover:text-foreground disabled:opacity-50" disabled>
                <ChevronLeft className="h-4 w-4" />
              </button>
              <span className="px-2 font-medium">Page 1 of 1</span>
              <button className="p-1 rounded text-muted-foreground hover:bg-[#27272a] hover:text-foreground disabled:opacity-50" disabled>
                <ChevronRight className="h-4 w-4" />
              </button>
              <button className="p-1 rounded text-muted-foreground hover:bg-[#27272a] hover:text-foreground disabled:opacity-50" disabled>
                <ChevronsRight className="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
