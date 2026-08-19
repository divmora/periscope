import type { APIResponse } from "../types/api";
import { fetchAPI } from "./client";

export interface Project {
  iid: string;
  name: string;
  storage_type: string;
  storage_bucket: string;
  storage_region: string;
  storage_endpoint: string;
  prune_enabled: boolean;
  created_at: string;
  updated_at: string;
  // These are only returned from the config endpoint, never the list endpoint
  source_dsn?: string;
  storage_access_key?: string;
  storage_secret_key?: string;
}

export const ProjectsAPI = {
  /**
   * List all projects the user has access to.
   */
  listProjects: async (): Promise<APIResponse<Project[]>> => {
    return fetchAPI<Project[]>("/projects");
  },

  /**
   * Fetch a project's sensitive configuration.
   */
  getProjectConfig: async (projectId: string): Promise<APIResponse<Project>> => {
    return fetchAPI<Project>(`/projects/${projectId}/config`);
  },

  /**
   * Create a new project tenant.
   */
  createProject: async (data: Partial<Project>): Promise<APIResponse<Project>> => {
    return fetchAPI<Project>("/projects", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  /**
   * Update an existing project's configuration.
   */
  updateProjectConfig: async (projectId: string, data: Partial<Project>): Promise<APIResponse<Project>> => {
    return fetchAPI<Project>(`/projects/${projectId}/config`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  },
};
