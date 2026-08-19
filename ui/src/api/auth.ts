import { APIResponse } from "../types/api";
import { fetchAPI } from "./client";
import { Role } from "../lib/constants";

export interface User {
  iid: string;
  username: string;
  email: string;
  global_role: Role;
}

export const AuthAPI = {
  login: async (username: string, password: string): Promise<APIResponse<User>> => {
    return fetchAPI<User>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    });
  },

  logout: async (): Promise<APIResponse<{ message: string }>> => {
    return fetchAPI<{ message: string }>("/auth/logout", {
      method: "POST",
    });
  },

  me: async (): Promise<APIResponse<User>> => {
    return fetchAPI<User>("/auth/me");
  },
};
