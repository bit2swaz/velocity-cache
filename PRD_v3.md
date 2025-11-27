# Product Requirements Document: VelocityCache v3.0 (Infrastructure Edition)

| Meta Data | Details |
| :--- | :--- |
| **Project** | VelocityCache |
| **Version** | v3.0 (Self-Hosted / Infrastructure) |
| **Status** | IMPLEMENTED |
| **Owner** | Aditya Mishra (bit2swaz) |
| **Objective** | Build a high-performance, stateless, self-hostable remote build cache for monorepos. |
| **License** | Open Source (MIT/Apache 2.0) |

---

## 1. Executive Summary

### 1.1. The Pivot
VelocityCache v3.0 shifts from a managed SaaS product to a distributed infrastructure component. It is designed to be deployed inside a company's private VPC (Virtual Private Cloud), ensuring 100% data sovereignty, zero external dependencies, and maximum speed.

### 1.2. The Core Philosophy
**"The Network is the Cache."**
The server acts as a lightweight traffic controller. It does not act as a bandwidth bottleneck. Instead, it orchestrates direct, high-bandwidth streams between the Build Agent (CLI) and the Object Storage (S3/MinIO) utilizing Presigned URLs.

### 1.3. Target User
Platform Engineers & DevOps Leads who manage CI/CD pipelines for large monorepos (Turborepo, Nx, Bazel) and require a secure, on-premise caching solution without the complexity of managing a database.

---

## 2. System Architecture

The system follows a **Stateless Client-Server model** with **Pluggable Storage Drivers**.

### 2.1. High-Level Data Flow (The "Vending Machine" Pattern)
Instead of handling heavy file uploads directly, the Server "vends" a ticket (URL) to the CLI. The CLI then interacts directly with the storage.

**Scenario: Uploading an Artifact (Cache Miss)**
1.  **CLI**: Hashes inputs $\rightarrow$ `abc-123`.
2.  **CLI**: Sends `POST /v1/negotiate` with body `{"hash": "abc-123", "action": "upload"}`.
3.  **Server**: Checks Storage Driver.
    *   If object exists: Returns `{"status": "skipped"}` (Immutability).
    *   If new: Calls Driver to generate a URL.
4.  **Server**: Returns `{"status": "upload_needed", "url": "https://s3.aws.../abc-123"}`.
5.  **CLI**: Reads response.
6.  **CLI**: Performs a `PUT` request directly to the provided `url` with the file content.

### 2.2. Components
1.  **Velocity Agent (CLI)**: A Go binary running in CI/Local. Handles hashing, graph execution, and Client-Side Orchestration (handling the negotiation response).
2.  **Velocity Gateway (Server)**: A stateless Go server. Handles authentication, generates presigned tickets, and enforces the "First Write Wins" policy.
3.  **Storage Backend**:
    *   **S3/MinIO/GCS**: Direct upload/download via Presigned URLs.
    *   **Local Disk**: Upload/download via a special Proxy Endpoint on the Gateway.

---

## 3. Core Functional Requirements

### 3.1. The Velocity Server (`cmd/server`)

#### A. Storage Driver Interface
The server must be storage-agnostic. Defined in `pkg/storage`:

```go
type Driver interface {
    // Generates a URL for the client to PUT data to
    GetUploadURL(ctx context.Context, key string) (string, error)

    // Generates a URL for the client to GET data from
    GetDownloadURL(ctx context.Context, key string) (string, error)

    // Checks metadata (HeadObject) to see if key exists
    Exists(ctx context.Context, key string) (bool, error)
}
```

#### B. Configuration (12-Factor)
No config files. Pure Environment Variables.

*   `VC_PORT`: Port to listen on (Default: `8080`).
*   `VC_AUTH_TOKEN`: Shared secret for Bearer Auth.
*   `VC_STORAGE_DRIVER`: `s3` or `local`.
*   `VC_S3_BUCKET`: Bucket name (for S3 driver).
*   `VC_S3_REGION`: AWS Region (for S3 driver).
*   `VC_S3_ENDPOINT`: Custom S3 Endpoint (e.g. for MinIO).
*   `VC_LOCAL_ROOT`: Directory path (for Local driver).
*   `VC_BASE_URL`: Public URL of the server (for Local driver).

#### C. Immutability Logic (Security)
*   **Rule**: "First Write Wins."
*   **Implementation**: In the Negotiate handler, the server MUST call `driver.Exists(key)` first.
*   **Why**: Prevents cache poisoning. If a valid cache exists for a hash, no one can overwrite it.

#### D. Observability
*   Expose `/metrics` (Prometheus format).
*   Track: `vc_cache_hits`, `vc_cache_misses`, `vc_negotiation_latency`.

### 3.2. The Velocity CLI (`cmd/velocity`)

#### A. Protocol Handling
*   **Negotiator**: A new internal client module that calls `/v1/negotiate`.
*   **Transfer Agent**: A module that takes the `url` and `method` from the negotiator and executes the HTTP transfer (streaming data from tar).

#### B. Configuration (`velocity.yml`)
Move from JSON to YAML for readability.

```yaml
version: 1
remote:
  enabled: true
  url: "https://cache.internal.corp"
  token: "${VC_AUTH_TOKEN}" # Env var expansion
pipeline:
  build:
    outputs: ["dist/**"]
    depends_on: ["^build"]
```

---

## 4. API Specification (v3 Protocol)

All endpoints (except `/health`) require `Authorization: Bearer <token>`.

### 4.1. Negotiate (The Vending Machine)
*   **Endpoint**: `POST /v1/negotiate`
*   **Request Body**:
    ```json
    {
      "hash": "sha256-abc123...",
      "action": "download" | "upload"
    }
    ```
*   **Response (Action: Download)**:
    *   Found: `{"status": "found", "url": "https://s3..."}`
    *   Missing: `{"status": "missing"}`
*   **Response (Action: Upload)**:
    *   Exists: `{"status": "skipped"}`
    *   New: `{"status": "upload_needed", "url": "https://s3..."}`

### 4.2. Proxy Endpoint (Local Driver Only)
*   **Endpoint**: `PUT /v1/proxy/{key}` and `GET /v1/proxy/{key}`
*   **Behavior**:
    *   Only enabled if `VC_STORAGE_DRIVER=local`.
    *   The `GetUploadURL` driver method returns this internal URL (e.g., `https://velocity.corp/v1/proxy/{key}`).
    *   **Implementation Detail**: Must use `io.Copy` to stream data to/from disk. Do not load into RAM.

### 4.3. System Health
*   **Endpoint**: `GET /health`
*   **Response**: `200 OK {"status": "up"}`

---

## 5. Security Strategy

### 5.1. Content-Addressable Storage (CAS)
Cache Keys are `SHA-256(Inputs)`. It is mathematically impossible to retrieve the "wrong" cache for a set of inputs unless the inputs themselves change.

### 5.2. Path Traversal Defense
Strict Regex on keys: `^[a-zA-Z0-9-_]+$`. Blocks `../../etc/passwd`.

### 5.3. Authentication
Shared Bearer Token. Simple, stateless, effective for internal infrastructure.
