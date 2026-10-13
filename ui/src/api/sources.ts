import type { APIResponse } from "../types/api";
import { fetchAPI } from "./client";

export interface ProjectSource {
  iid: string;
  project_iid: string;
  name: string;
  source_type: "laravel_telescope" | "aurora_das" | "postgres_audit" | "opentelemetry" | "generic_db";
  status: "active" | "paused" | "error";
  dsn?: string;
  config: Record<string, any>;
  created_at: string;
  updated_at: string;
}

export const SourcesAPI = {
  listSources: async (projectId: string): Promise<APIResponse<ProjectSource[]>> => {
    return fetchAPI<ProjectSource[]>(`/projects/${projectId}/sources`);
  },

  createSource: async (projectId: string, data: Partial<ProjectSource>): Promise<APIResponse<ProjectSource>> => {
    return fetchAPI<ProjectSource>(`/projects/${projectId}/sources`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  deleteSource: async (projectId: string, sourceIid: string): Promise<APIResponse<{ message: string }>> => {
    return fetchAPI<{ message: string }>(`/projects/${projectId}/sources/${sourceIid}`, {
      method: "DELETE",
    });
  },
};
