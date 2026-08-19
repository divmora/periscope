import type { APIResponse } from "../types/api";

const API_BASE_URL = "/api"; // Adjust this based on your Vite proxy or actual backend URL

export async function fetchAPI<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<APIResponse<T>> {
  const url = `${API_BASE_URL}${endpoint}`;
  
  const headers = {
    "Content-Type": "application/json",
    ...options.headers,
  };

  try {
    const response = await fetch(url, { ...options, headers });
    
    // Parse the standard JSON envelope
    const payload = (await response.json()) as APIResponse<T>;

    if (!response.ok || !payload.success) {
      throw new Error(payload.error?.message || "An unknown API error occurred");
    }

    return payload;
  } catch (error: any) {
    // If it's a network error or parsing error, we format it into our standard envelope
    console.error("API Fetch Error:", error);
    throw error;
  }
}
