# VelocityCache v3.0

**Stateless, Self-Hosted Remote Cache for High-Performance Monorepos.**

VelocityCache is a distributed infrastructure component designed to accelerate CI/CD pipelines by caching build artifacts. Unlike SaaS solutions, VelocityCache is designed to be deployed inside your private VPC, ensuring **100% data sovereignty**, **zero external dependencies**, and **maximum speed**.

"The Network is the Cache."

---

## Why Self-Hosted?

*   **Data Sovereignty:** Your code and artifacts never leave your VPC.
*   **No SaaS Lock-in:** You own the infrastructure. No per-seat pricing or bandwidth overages.
*   **Performance:** Designed for high-bandwidth internal networks (e.g., AWS VPC, K8s Clusters).
*   **Simplicity:** Stateless architecture. No database to manage. Just a binary and an Object Store.

## Architecture: The "Vending Machine" Pattern

VelocityCache abandons the traditional "proxy everything" model in favor of a high-performance "Vending Machine" pattern. The server acts as a lightweight traffic controller, while the heavy lifting of data transfer happens directly between the CLI and your Object Storage.

### High-Level Data Flow

1.  **Negotiation**: The Velocity CLI sends a hash of the inputs to the Server.
2.  **Vending**:
    *   **Cache Hit**: Server confirms existence.
    *   **Cache Miss**: Server generates a **Presigned URL** (S3/MinIO) and returns it to the CLI.
3.  **Transfer**: The CLI streams the artifact directly to/from the Object Storage using the presigned URL.

This architecture ensures the Server is never a bandwidth bottleneck.

### Components

*   **Velocity Agent (CLI)**: Runs in CI/Local. Handles hashing, graph execution, and direct storage transfers.
*   **Velocity Gateway (Server)**: Stateless Go server. Handles authentication, generates tickets, and enforces security policies.
*   **Storage Backend**: S3, MinIO, GCS, or Local Disk (via Proxy).

## Installation

### 1. The Local Cloud (Docker Compose)

The easiest way to spin up the entire stack (Server + MinIO) for testing.

```yaml
version: '3.8'
services:
  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: admin
      MINIO_ROOT_PASSWORD: password
    ports:
      - "9000:9000"
      - "9001:9001"

  velocity:
    image: bit2swaz/velocity-server:v3
    environment:
      VC_PORT: 8080
      VC_AUTH_TOKEN: secret-token
      VC_STORAGE_DRIVER: s3
      VC_S3_ENDPOINT: http://minio:9000
      VC_S3_BUCKET: velocity-cache
      VC_S3_REGION: us-east-1
      AWS_ACCESS_KEY_ID: admin
      AWS_SECRET_ACCESS_KEY: password
      AWS_REGION: us-east-1
    ports:
      - "8080:8080"
    depends_on:
      - minio
```

### 2. Binary Installation

Download the latest release for your platform.

```bash
# Server
./velocity-server

# CLI
./velocity-cli run build
```

## Configuration

### Server Configuration (Environment Variables)

The server follows the 12-Factor App methodology.

| Variable | Description | Default |
| :--- | :--- | :--- |
| `VC_PORT` | Port to listen on | `8080` |
| `VC_AUTH_TOKEN` | Shared secret for Bearer Auth | - |
| `VC_STORAGE_DRIVER` | Storage backend (`s3` or `local`) | - |
| `VC_S3_BUCKET` | Bucket name (for S3 driver) | - |
| `VC_S3_REGION` | AWS Region (for S3 driver) | - |
| `VC_LOCAL_ROOT` | Directory path (for Local driver) | - |

### Client Configuration (`velocity.yml`)

VelocityCache v3.0 uses a clean YAML configuration file in your project root.

```yaml
version: 1
remote:
  enabled: true
  url: "http://localhost:8080"
  token: "${VC_AUTH_TOKEN}" # Supports env var expansion

pipeline:
  build:
    command: "npm run build"
    inputs:
      - "src/**"
      - "package.json"
    outputs:
      - "dist/**"
    depends_on:
      - "^build" # Topological dependency
```

## Security: First Write Wins

VelocityCache implements a strict **Immutability Policy** to prevent cache poisoning.

*   **Rule**: Once a cache key is written, it cannot be overwritten.
*   **Mechanism**: During the `negotiate` phase, the server checks the storage driver. If the key exists, it returns `skipped`, and the CLI will not attempt an upload.
*   **Benefit**: Guarantees that a specific input hash always resolves to the exact same artifact, regardless of race conditions in CI.

## 📊 Observability

The server exposes a `/metrics` endpoint compatible with Prometheus.

*   `vc_cache_hits`: Total cache hits.
*   `vc_cache_misses`: Total cache misses.
*   `vc_negotiation_latency`: Time taken to negotiate tickets.
