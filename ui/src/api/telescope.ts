import type { APIResponse } from "../types/api";
import type { TelescopeRequest } from "../types/telescope";
import { fetchAPI } from "./client";

export interface TelescopeException {
  id: string;
  class: string;
  message: string;
  time: string;
}

export interface TelescopeQuery {
  id: string;
  sql: string;
  duration: string;
  time: string;
}

export interface TelescopeLog {
  id: string;
  level: string;
  message: string;
  time: string;
}

export const TelescopeAPI = {
  getRequests: async (projectId: string, page = 1): Promise<APIResponse<TelescopeRequest[]>> => {
    return fetchAPI<TelescopeRequest[]>(`/projects/${projectId}/telescope/requests?page=${page}`);
  },

  getExceptions: async (projectId: string, page = 1): Promise<APIResponse<TelescopeException[]>> => {
    return fetchAPI<TelescopeException[]>(`/projects/${projectId}/telescope/exceptions?page=${page}`);
  },

  getQueries: async (projectId: string, page = 1): Promise<APIResponse<TelescopeQuery[]>> => {
    return fetchAPI<TelescopeQuery[]>(`/projects/${projectId}/telescope/queries?page=${page}`);
  },

  getLogs: async (projectId: string, page = 1): Promise<APIResponse<TelescopeLog[]>> => {
    return fetchAPI<TelescopeLog[]>(`/projects/${projectId}/telescope/logs?page=${page}`);
  },

  getRequestDetails: async (projectId: string, requestId: string): Promise<APIResponse<TelescopeRequest>> => {
    return fetchAPI<TelescopeRequest>(`/projects/${projectId}/telescope/requests/${requestId}`);
  }
};
