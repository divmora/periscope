export interface APIError {
  code: string;
  message: string;
  details?: Record<string, string>;
}

export interface APIMeta {
  total: number;
  per_page: number;
  current_page: number;
  last_page: number;
}

export interface APIResponse<T> {
  success: boolean;
  data: T | null;
  error: APIError | null;
  meta?: APIMeta;
}
