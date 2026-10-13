import type { APIResponse } from "../types/api";
import { fetchAPI } from "./client";

export interface SystemStorageConfig {
  storage_type: string;
  storage_bucket: string;
  storage_endpoint: string;
  storage_region: string;
  is_default_ready: boolean;
}

export const SystemAPI = {
  getSystemStorage: async (): Promise<APIResponse<SystemStorageConfig>> => {
    return fetchAPI<SystemStorageConfig>("/system/storage");
  },
};
