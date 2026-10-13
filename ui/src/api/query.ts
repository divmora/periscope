import type { APIResponse } from "../types/api";
import { fetchAPI } from "./client";

export interface QueryExecutionResult {
  columns: string[];
  rows: any[][];
  duration_ms: number;
  row_count: number;
  truncated: boolean;
}

export const QueryAPI = {
  executeSQL: async (projectId: string, query: string): Promise<APIResponse<QueryExecutionResult>> => {
    return fetchAPI<QueryExecutionResult>(`/projects/${projectId}/query/execute`, {
      method: "POST",
      body: JSON.stringify({ query }),
    });
  },
};
