export interface TelescopeRequest {
  id: string; // The sequence ID or UUID
  method: string;
  path: string;
  status: number;
  duration: string; // e.g., "120ms"
  time: string; // Formatted datetime string
  // Extend this interface later with headers, payload, etc.
}
