import type { APIResponse } from "../types/api";
import { fetchAPI } from "./client";

export interface ProjectTeam {
  iid: string;
  project_iid: string;
  name: string;
  description: string;
  role: "viewer" | "member" | "admin";
  allowed_sources: string[];
  member_count: number;
  created_at: string;
  updated_at: string;
}

export interface TeamMember {
  user_iid: string;
  username: string;
  email: string;
  created_at: string;
}

export const TeamsAPI = {
  listTeams: async (projectId: string): Promise<APIResponse<ProjectTeam[]>> => {
    return fetchAPI<ProjectTeam[]>(`/projects/${projectId}/teams`);
  },

  createTeam: async (
    projectId: string,
    data: { name: string; description?: string; role: "viewer" | "member" | "admin"; allowed_sources?: string[] }
  ): Promise<APIResponse<ProjectTeam>> => {
    return fetchAPI<ProjectTeam>(`/projects/${projectId}/teams`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  updateTeam: async (
    projectId: string,
    teamId: string,
    data: { name: string; description?: string; role: "viewer" | "member" | "admin"; allowed_sources?: string[] }
  ): Promise<APIResponse<{ message: string }>> => {
    return fetchAPI<{ message: string }>(`/projects/${projectId}/teams/${teamId}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  },

  deleteTeam: async (projectId: string, teamId: string): Promise<APIResponse<{ message: string }>> => {
    return fetchAPI<{ message: string }>(`/projects/${projectId}/teams/${teamId}`, {
      method: "DELETE",
    });
  },

  listTeamMembers: async (projectId: string, teamId: string): Promise<APIResponse<TeamMember[]>> => {
    return fetchAPI<TeamMember[]>(`/projects/${projectId}/teams/${teamId}/members`);
  },

  addTeamMember: async (
    projectId: string,
    teamId: string,
    userIdentifier: string
  ): Promise<APIResponse<{ message: string }>> => {
    return fetchAPI<{ message: string }>(`/projects/${projectId}/teams/${teamId}/members`, {
      method: "POST",
      body: JSON.stringify({ user_identifier: userIdentifier }),
    });
  },

  removeTeamMember: async (
    projectId: string,
    teamId: string,
    userIid: string
  ): Promise<APIResponse<{ message: string }>> => {
    return fetchAPI<{ message: string }>(`/projects/${projectId}/teams/${teamId}/members/${userIid}`, {
      method: "DELETE",
    });
  },
};
