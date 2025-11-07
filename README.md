# velocity-cache ⚡

[![build status](https://img.shields.io/badge/build-passing-brightgreen)](https://github.com/bit2swaz/velocity-cache) [![npm version](https://img.shields.io/npm/v/velocity-cache)](https://www.npmjs.com/package/velocity-cache) [![license: mit](https://img.shields.io/badge/license-mit-blue.svg)](https://opensource.org/licenses/mit)

a blazing-fast, open-source distributed build cache. built in go to accelerate your monorepo and ci/cd pipelines.

---

![velocity-cache demo gif](termtosvg_mtmm_9z0.svg)

_a 10-minute monorepo build (cache miss) becomes a 10-second build (cache hit)._

## why velocity-cache?

software teams waste millions of dollars and thousands of developer-hours waiting for slow builds, tests, and ci jobs. this "wait time" is a direct drain on productivity and happiness.

`velocity-cache` fixes this.

it's a high-performance build system, written in go, that wraps your existing tasks. it understands your project's structure, even in a complex monorepo. it intelligently "fingerprints" your tasks. if it's seen that exact state before—on your machine, a teammate's, or in ci—it downloads the pre-computed result from a **shared cache** in seconds instead of re-running the task.

the first person pays the time cost. everyone else benefits instantly.

## features

- **true monorepo support:** understands task dependencies (`dependsOn`) and topological caching (`^`) just like turborepo.
- **blazing fast:** built in go for high-performance, concurrent hashing and **parallel task execution**.
- **language-agnostic:** first-class support for javascript, rust, go, and python projects.
- **smart `init`:** auto-detects your project type (`turborepo`, `cargo`, `go.mod`, `poetry.lock`) to generate a config for you.
- **distributed:** uses a **free public cache by default**, or your own private s3/r2 bucket.
- **smart:** respects your `.gitignore` and hashes file contents, not just timestamps.

## installation & usage

the easiest way to use `velocity-cache` is with `npx`.

### 1. `npx velocity-cache init`

in the root of your project, run the `init` command.

```bash
npx velocity-cache init
````

`velocity-cache` will inspect your project:

  * if it finds a `turbo.json`, `cargo.toml`, `go.mod`, or `requirements.txt`, it will **auto-generate** a `velocity.config.json` for you based on those settings.
  * if not, it will create a generic template.

**you must review this file** to ensure your `inputs` and `outputs` are correct.

### 2\. `npx velocity-cache run <task-name>`

this is the main command. it runs a task (like `build`) for all packages in your monorepo, in the correct dependency order.

```bash
# runs the 'build' task for all packages
npx velocity-cache run build
```

to run a task for a **single package** and its dependencies, use the `-p` (`--package`) flag:

```bash
# runs the 'build' task for only 'apps/web' (and any of its dependencies)
npx velocity-cache run build -p "apps/web"
```

### 3\. `npx velocity-cache clean`

if you ever need to clear your *local* cache:

```bash
npx velocity-cache clean
```

-----

## caching behavior: public vs. private

`velocity-cache` supports two remote caching modes:

### 1\. public community cache (default) 🌍

  * **how it works:** if you **do not** provide any s3/r2 credentials (environment variables), `velocity-cache` will automatically use a free, shared, anonymous public cache hosted by the `velocity-cache` project.
  * **pros:**
      * **zero setup:** works instantly after `init`.
      * **free:** no cost for storage or bandwidth.
  * **cons:**
      * **public:** your build artifacts are uploaded anonymously to a public bucket. **do not use this if your build outputs contain sensitive information.**
      * **no guarantees:** cache entries expire after 7 days. intended for open-source projects and trying out `velocity-cache`.

### 2\. private s3/r2 bucket (recommended for teams) 🔒

  * **how it works:** if you **do** provide s3/r2 credentials via environment variables, `velocity-cache` will use your configured private bucket.
  * **pros:**
      * **secure:** your build artifacts stay within your control.
      * **reliable:** you control the cache retention.
  * **cons:**
      * **requires setup:** you need to create an s3/r2 bucket and api token.
      * **costs:** you pay for storage/operations (though r2 is very generous).

**to use a private bucket:**

1.  **create an s3/r2 bucket** (e.g., `my-company-build-cache`).

2.  **create an api token/access key** with read/write permissions for that bucket.

3.  **set these environment variables** in your local machine or ci environment:

      * **for cloudflare r2:**
        ```bash
        export R2_ACCOUNT_ID="your_account_id"
        export R2_ACCESS_KEY_ID="your_access_key_id"
        export R2_SECRET_ACCESS_KEY="your_secret_access_key"
        ```
      * **for aws s3:**
        ```bash
        export AWS_ACCESS_KEY_ID="your_access_key_id"
        export AWS_SECRET_ACCESS_KEY="your_secret_access_key"
        export AWS_REGION="your_bucket_region" # e.g., us-east-1
        ```

4.  **update your `velocity.config.json`** with your bucket name and region.

`velocity-cache` will automatically detect these environment variables and switch to using your private bucket.

-----

## how monorepo caching works 💡

`velocity-cache` understands dependencies. this is the most important concept.

  * `"dependsOn": ["lint"]`: (local dependency) this task will wait for the `lint` task *in the same package* to finish first.
  * `"dependsOn": ["^build"]`: (topological dependency) this is the magic. this task will wait for the `build` task in all of its *internal package dependencies* (defined in `package.json`) to finish first.

the cache key for a task (e.g., `apps/web#build`) is generated by hashing its own inputs *plus* the final cache keys of all its dependencies.

**this means if a file in your `libs/ui` package changes, `libs/ui#build` gets a new hash. this new hash causes `apps/web#build` to get a new hash, correctly busting the cache all the way up the chain.**

## configuration (`velocity.config.json`)

`velocity-cache` now uses a monorepo-aware config.

```json
{
  "$schema": "[https://velocitycache.dev/schema.json](https://velocitycache.dev/schema.json)",
  "remote_cache": {
    "enabled": true,
    // required only if using a private cache
    "bucket": "your-private-bucket-name",
    "region": "auto" // e.g., us-east-1 for aws, auto for r2
  },
  "packages": [
    "apps/*",
    "libs/*"
  ],
  "tasks": {
    "build": {
      "command": "npm run build",
      "dependsOn": ["^build"],
      "inputs": ["src/**/*", "package.json"],
      "outputs": ["dist/", ".next/"]
    },
    "test": {
      "command": "npm run test",
      "dependsOn": ["build"],
      "inputs": ["src/**/*.test.ts"],
      "outputs": ["coverage/"]
    },
    "lint": {
      "command": "npm run lint",
      "dependsOn": [],
      "inputs": ["src/**/*.ts"]
    }
  }
}
```

  * **`packages`**: an array of globs to find your individual packages (looks for `package.json`, `cargo.toml`, etc.).
  * **`tasks`**: a map of task definitions.
      * **`"build"`**: the name of the task you'll run (e.g., `npx velocity-cache run build`).
      * **`command`**: the *actual* shell command to run on a cache miss.
      * **`dependsOn`**: **critical.** an array of tasks this task depends on (see "how monorepo caching works" above).
      * **`inputs`**: **critical.** list of files/globs `velocity` will hash. changes here trigger a cache miss.
      * **`outputs`**: **critical.** list of directories created by `command`. these are cached.
      * **`env_keys`**: list of environment variables included in the hash.

## contributing

pull requests are welcome\! for major changes, please open an issue first to discuss what you would like to change.

## license

[mit](https://opensource.org/licenses/mit)
