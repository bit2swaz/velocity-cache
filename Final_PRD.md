# Product Requirements Document: VelocityCache v3.1.0

## Meta Data

| Field     | Value                                                                                                                      |
| --------- | -------------------------------------------------------------------------------------------------------------------------- |
| Project   | VelocityCache                                                                                                              |
| Version   | v3.1.0 (Infrastructure Edition)                                                                                            |
| Status    | COMPLETED / GOLD MASTER                                                                                                    |
| Owner     | Aditya Mishra (bit2swaz)                                                                                                   |
| Objective | Provide a high-performance, stateless, self-hostable remote build cache for monorepos with enterprise-grade observability. |
| License   | MIT                                                                                                                        |

---

## 1. Executive Summary

### 1.1 Vision

To eliminate redundant compute in software development by turning the build cache into a commodity infrastructure component. VelocityCache decouples the caching logic from the storage layer, allowing engineering teams to own their data fully.

### 1.2 Pivot

VelocityCache v3.0+ moves away from a SaaS model. It is offered as Infrastructure as Code — a binary deployed inside the user's VPC to provide a private, high-bandwidth caching layer compatible with any build system.

### 1.3 Core Philosophy — *"The Network is the Cache"*

Traditional caches proxy uploads through a central server, causing CPU and I/O bottlenecks. VelocityCache uses a "Vending Machine" architecture where the server issues Presigned URLs that allow the CLI to stream artifacts directly to S3/MinIO, bypassing the server.

---

## 2. System Architecture

Stateless Client-Server architecture with pluggable storage drivers.

### 2.1 High-Level Data Flow

```
sequenceDiagram
    participant CLI as Velocity Agent
    participant Server as Velocity Gateway
    participant S3 as S3 / MinIO

    Note over CLI, Server: 1. Negotiation Phase
    CLI->>Server: POST /v1/negotiate (Hash + Action)
    
    alt Cache Hit (Download)
        Server->>S3: Check Existence
        Server->>S3: Generate Presigned GET URL
        Server-->>CLI: {"status": "found", "url": "..."}
    else Cache Miss (Upload)
        Server->>S3: Check Existence
        alt Exists (Immutability Trigger)
            Server-->>CLI: {"status": "skipped"}
        else New Artifact
            Server->>S3: Generate Presigned PUT URL
            Server-->>CLI: {"status": "upload_needed", "url": "..."}
        end
    end

    Note over CLI, S3: 2. Transfer Phase
    CLI->>S3: Stream Artifact directly to/from Storage
```

### 2.2 Logical Components

| Component                 | Role                                                             | Tech Stack            |
| ------------------------- | ---------------------------------------------------------------- | --------------------- |
| Velocity Agent (CLI)      | Runs in CI/local. Handles hashing, DAG execution, and transfers. | Go (Cobra, Viper)     |
| Velocity Gateway (Server) | Stateless server for auth, presigned URL vending, metrics.       | Go (Chi), Docker      |
| Storage Backend           | Persistent artifact storage                                      | S3, MinIO, Local Disk |

---

## 3. Core Functional Requirements

### 3.1 Velocity Server (`cmd/server`)

#### A. Storage Driver Interface

Pluggable driver system (`pkg/storage`) supporting:

* S3 driver: AWS SDK v2 for Presigned URLs
* Local driver: Proxy URL streaming pattern

#### B. Observability (New in v3.1)

Prometheus metrics exposed via `/metrics`:

* `vc_cache_operations_total`
* `vc_http_duration_seconds`
* `vc_proxy_bytes_total` (local driver only)

#### C. Cache Eviction — *"The Janitor"* (New in v3.1)

* S3 mode → relies on Lifecycle TTL
* Local mode → hourly scan to delete artifacts older than `VC_RETENTION_DAYS`

#### D. Security — *"First Write Wins"*

Artifacts are immutable:

* If hash exists, server returns `skipped`
* Prevents cache poisoning

---

### 3.2 Velocity CLI (`cmd/velocity`)

#### A. Graph Engine (Monorepo Support)

* Topological hashing: dependency hash propagation
* Concurrent task execution using goroutines

#### B. Language Agnosticism

`init` auto-generates config via heuristic detection:

* Turborepo (`turbo.json`)
* Rust (`Cargo.toml`)
* Go (`go.mod`)
* Python (`poetry.lock` / `requirements.txt`)

#### C. Protocol Handling

* Negotiator: JSON handshake logic
* Transfer agent: executes HTTP PUT/GET against presigned URLs

---

## 4. API Specification

All endpoints except `/health` and `/metrics` require:
`Authorization: Bearer <VC_AUTH_TOKEN>`

### 4.1 Control Plane

`POST /v1/negotiate`

Request body:

```json
{"hash": "sha256...", "action": "upload" | "download"}
```

Responses:

```json
{"status": "found", "url": "..."}
{"status": "missing"}
{"status": "upload_needed", "url": "..."}
{"status": "skipped"}
```

### 4.2 Data Plane (Local Driver Only)

| Method | Endpoint               | Description           |
| ------ | ---------------------- | --------------------- |
| PUT    | `/v1/proxy/blob/{key}` | Stream upload to disk |
| GET    | `/v1/proxy/blob/{key}` | Stream file from disk |

### 4.3 Ops Plane

| Endpoint       | Description                |
| -------------- | -------------------------- |
| `GET /health`  | Returns `{"status": "up"}` |
| `GET /metrics` | Prometheus metrics         |

---

## 5. Configuration Reference

### 5.1 Server Environment Variables

| Variable              | Description                   | Default  |
| --------------------- | ----------------------------- | -------- |
| VC_PORT               | Port to listen on             | 8080     |
| VC_AUTH_TOKEN         | Shared secret for bearer auth | Required |
| VC_STORAGE_DRIVER     | `s3` or `local`               | local    |
| VC_RETENTION_DAYS     | Retention for local only      | 7        |
| VC_S3_BUCKET          | S3 bucket name                | -        |
| VC_S3_REGION          | AWS region                    | -        |
| AWS_ACCESS_KEY_ID     | Credential                    | -        |
| AWS_SECRET_ACCESS_KEY | Credential                    | -        |
| AWS_ENDPOINT_URL      | Override (MinIO/R2)           | -        |

### 5.2 Client Configuration (`velocity.yml`)

```yml
version: 1
project_id: "production-monorepo"

remote:
  enabled: true
  url: "https://cache.internal.corp"
  token: "${CI_CACHE_TOKEN}"

pipeline:
  build:
    command: "npm run build"
    inputs: ["src/**", "package.json"]
    outputs: ["dist/**"]
    depends_on: ["^build"]
```

---

## 6. Deployment & DevOps

### 6.1 Docker Image

* Base: `gcr.io/distroless/static-debian11`
* Security: Non-root, no shell
* Size: < 25MB compressed

### 6.2 Kubernetes / Docker Compose

Deployed as:

* Single Deployment → Velocity Gateway
* StatefulSet → MinIO (or cloud S3)
* Includes Prometheus in reference compose bundle

---

## 7. Project History & Pivot Logic

| Version               | Characteristics                   | Limitation                                                       |
| --------------------- | --------------------------------- | ---------------------------------------------------------------- |
| v1.0 (MVP)            | Direct-to-S3 from CLI             | Required distributing AWS keys to every developer                |
| v2.0 (SaaS)           | Frontend + Postgres               | Operational complexity, DB latency, privacy issues               |
| v3.0 (Infrastructure) | Stateless server + Presigned URLs | Current direction: max performance, zero DB, full data ownership |