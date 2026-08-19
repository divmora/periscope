# API Standards & Response Format

To ensure consistency across the entire Periscope ecosystem, all HTTP endpoints must return data using a standardized JSON envelope. 

This prevents front-end clients from having to guess the shape of the response or write multiple different error-handling branches.

## The Standard JSON Envelope

Every HTTP response must adhere to the following structure:

```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "meta": { ... }
}
```

### Fields:
- **`success`** `(boolean)`: `true` if the HTTP status code is 2xx, `false` otherwise.
- **`data`** `(object | array | null)`: The actual payload requested. Will be `null` if an error occurs.
- **`error`** `(object | null)`: Null on success. On failure, contains a structured error object (see below).
- **`meta`** `(object | null)`: Optional metadata, typically used for pagination (e.g., `total`, `per_page`, `current_page`).

## Error Structure

When `success` is false, the `error` object must follow this format:

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Invalid project ID provided.",
    "details": {
      "project_id": "Must be an integer"
    }
  }
}
```

### Fields:
- **`code`**: A machine-readable string constant (e.g., `NOT_FOUND`, `UNAUTHORIZED`, `INTERNAL_ERROR`).
- **`message`**: A human-readable description of the error.
- **`details`**: An optional object containing field-specific validation errors or stack traces (only in dev mode).

## Go Implementation

The standard wrapper is implemented in the `internal/api` package. Always use the provided helper functions when returning data from an HTTP handler:

- `api.RespondJSON(w, http.StatusOK, data, meta)`
- `api.RespondError(w, http.StatusBadRequest, "VALIDATION_FAILED", "Invalid input", details)`
