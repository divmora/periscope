import type { APIResponse } from "../types/api";

export interface DASRecord {
  id: string;
  event_time: string;
  database_name: string;
  db_user: string;
  client_ip: string;
  client_app: string;
  command: string;
  command_text: string;
  execution_time_ms: number;
  status: "success" | "failure" | string;
  error_message?: string;
  pid?: number;
}

export interface DASFilterParams {
  search?: string;
  user?: string;
  ip?: string;
  database?: string;
  command?: string;
  status?: string;
  start_date?: string;
  end_date?: string;
  page?: number;
  per_page?: number;
  engine?: "direct" | "athena";
  source_iid?: string;
}

export interface DASStats {
  total_queries: number;
  total_errors: number;
  avg_duration_ms: number;
  active_users?: number;
  command_counts: Record<string, number>;
  engine: string;
}

export const DASAPI = {
  getActivity: async (projectId: string, params: DASFilterParams = {}): Promise<APIResponse<DASRecord[]>> => {
    const searchParams = new URLSearchParams();
    if (params.search) searchParams.set("search", params.search);
    if (params.user) searchParams.set("user", params.user);
    if (params.ip) searchParams.set("ip", params.ip);
    if (params.database) searchParams.set("database", params.database);
    if (params.command) searchParams.set("command", params.command);
    if (params.status) searchParams.set("status", params.status);
    if (params.start_date) searchParams.set("start_date", params.start_date);
    if (params.end_date) searchParams.set("end_date", params.end_date);
    if (params.page) searchParams.set("page", params.page.toString());
    if (params.per_page) searchParams.set("per_page", params.per_page.toString());
    if (params.engine) searchParams.set("engine", params.engine);
    if (params.source_iid) searchParams.set("source_iid", params.source_iid);

    const qs = searchParams.toString();
    const url = `/api/projects/${projectId}/das/activity${qs ? `?${qs}` : ""}`;
    const res = await fetch(url);
    if (!res.ok) {
      const errBody = await res.json().catch(() => null);
      throw new Error(errBody?.error?.message || `Failed to fetch DAS activity logs: ${res.statusText}`);
    }
    return res.json();
  },

  getStats: async (projectId: string, sourceIid?: string): Promise<APIResponse<DASStats>> => {
    const url = `/api/projects/${projectId}/das/stats${sourceIid ? `?source_iid=${sourceIid}` : ""}`;
    const res = await fetch(url);
    if (!res.ok) {
      const errBody = await res.json().catch(() => null);
      throw new Error(errBody?.error?.message || `Failed to fetch DAS statistics: ${res.statusText}`);
    }
    return res.json();
  },
};
