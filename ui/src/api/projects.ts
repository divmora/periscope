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
  source_retention_days?: number;
  lakehouse_retention_days?: number;
  compaction_enabled?: boolean;
  created_at: string;
  updated_at: string;
  // These are only returned from the config endpoint, never the list endpoint
  source_dsn?: string;
  storage_access_key?: string;
  storage_secret_key?: string;
}


export interface ProjectMember {
  user_iid: string;
  username: string;
  email: string;
  role: "viewer" | "member" | "admin";
  created_at: string;
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

  /**
   * Delete a project.
   */
  deleteProject: async (projectId: string): Promise<APIResponse<{ message: string }>> => {
    return fetchAPI<{ message: string }>(`/projects/${projectId}`, {
      method: "DELETE",
    });
  },

  /**
   * List all members of a project.
   */
  listMembers: async (projectId: string): Promise<APIResponse<ProjectMember[]>> => {
    return fetchAPI<ProjectMember[]>(`/projects/${projectId}/members`);
  },

  /**
   * Add or invite a member to a project.
   */
  addMember: async (
    projectId: string,
    userIdentifier: string,
    role: "viewer" | "member" | "admin"
  ): Promise<APIResponse<ProjectMember>> => {
    return fetchAPI<ProjectMember>(`/projects/${projectId}/members`, {
      method: "POST",
      body: JSON.stringify({ user_identifier: userIdentifier, role }),
    });
  },

  /**
   * Update a project member's role.
   */
  updateMemberRole: async (
    projectId: string,
    userIid: string,
    role: "viewer" | "member" | "admin"
  ): Promise<APIResponse<{ message: string }>> => {
    return fetchAPI<{ message: string }>(`/projects/${projectId}/members/${userIid}`, {
      method: "PUT",
      body: JSON.stringify({ role }),
    });
  },

  /**
   * Remove a member from a project.
   */
  removeMember: async (projectId: string, userIid: string): Promise<APIResponse<{ message: string }>> => {
    return fetchAPI<{ message: string }>(`/projects/${projectId}/members/${userIid}`, {
      method: "DELETE",
    });
  },
};
