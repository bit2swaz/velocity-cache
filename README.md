# velocity-cache ⚡

[![build status](https://img.shields.io/badge/build-passing-brightgreen)](https://github.com/bit2swaz/velocity-cache) [![npm version](https://img.shields.io/npm/v/velocity-cache)](https://www.npmjs.com/package/velocity-cache) [![license: mit](https://img.shields.io/badge/license-mit-blue.svg)](https://opensource.org/licenses/mit)

a blazing-fast, open-source distributed build cache. built in go to accelerate your ci/cd pipelines.

---

![velocity-cache demo gif](termtosvg_mtmm_9z0.svg)

_a 1-minute next.js build (cache miss) becomes a 1-second build (cache hit)._

## why velocity-cache?

software teams waste millions of dollars and thousands of developer-hours waiting for slow builds, tests, and ci jobs. this "wait time" is a direct drain on productivity and happiness.

`velocity-cache` fixes this.

it's a simple cli that wraps your existing build scripts (like `npm run build`). it intelligently "fingerprints" your project's state. if it's seen that exact state before—on your machine, or a teammate's, or in ci—it downloads the pre-computed result from a **shared cache** in seconds instead of re-running the task.

the first person pays the time cost. everyone else benefits instantly.

## features

- **blazing fast:** built in go for high-performance, concurrent hashing and caching.
- **distributed:** uses a **free public cache by default**, or your own private s3/r2 bucket. share cache across your entire team and ci/cd.
- **framework-agnostic:** works with any build script or framework (next.js, vite, remix, etc.).
- **smart:** respects your `.gitignore` and hashes file contents, not just timestamps.
- **zero config setup:** just run `npx velocity-cache init` and `npx velocity-cache run build`.

## installation & usage

the easiest way to use `velocity-cache` is with `npx`.

### 1. `npx velocity-cache init`

in the root of your project, run the `init` command.

```bash
npx velocity-cache init
```

this will create a `velocity.config.json` file. **you must edit this file** to tell `velocity-cache` about your specific project's build process (see configuration below).

### 2\. `npx velocity-cache run <script-name>`

this is the main command. instead of running `npm run build`, you now run:

```bash
npx velocity-cache run build
```

`velocity-cache` will:

1.  generate a unique hash of your project based on your config.
2.  check your local cache (`.velocity/cache/`).
3.  check the remote cache (public community cache by default, or your private bucket if configured).
4.  **if hit:** download the `.zip`, restore your output directories, and exit in \<1s.
5.  **if miss:** run your actual command (`npm run build`), then save the result locally and upload it to the remote cache.

### 3\. `npx velocity-cache clean`

if you ever need to clear your _local_ cache:

```bash
npx velocity-cache clean
```

### permanent installation (optional)

if you want to use `velocity-cache` without `npx` every time (e.g., in your `package.json` scripts), install it as a dev dependency:

```bash
npm install velocity-cache --save-dev
# or
yarn add velocity-cache --dev
# or
pnpm add velocity-cache --save-dev
```

then you can update your `package.json` like this:

```json
  "scripts": {
    "build": "velocity-cache run build:real",
    "build:real": "next build"
  }
```

## caching behavior: public vs. private

`velocity-cache` supports two remote caching modes:

### 1\. public community cache (default) 🌍

- **how it works:** if you **do not** provide any s3/r2 credentials (environment variables), `velocity-cache` will automatically use a free, shared, anonymous public cache hosted by the `velocity-cache` project.
- **pros:**
  - **zero setup:** works instantly after `init`.
  - **free:** no cost for storage or bandwidth.
- **cons:**
  - **public:** your build artifacts are uploaded anonymously to a public bucket. **do not use this if your build outputs contain sensitive information.**
  - **no guarantees:** cache entries expire after 7 days. intended for open-source projects and trying out `velocity-cache`.

### 2\. private s3/r2 bucket (recommended for teams) 🔒

- **how it works:** if you **do** provide s3/r2 credentials via environment variables, `velocity-cache` will use your configured private bucket.
- **pros:**
  - **secure:** your build artifacts stay within your control.
  - **reliable:** you control the cache retention.
- **cons:**
  - **requires setup:** you need to create an s3/r2 bucket and api token.
  - **costs:** you pay for storage/operations (though r2 is very generous).

**to use a private bucket:**

1.  **create an s3/r2 bucket** (e.g., `my-company-build-cache`).

2.  **create an api token/access key** with read/write permissions for that bucket.

3.  **set these environment variables** in your local machine or ci environment:

    - **for cloudflare r2:**
      ```bash
      export R2_ACCOUNT_ID="your_account_id"
      export R2_ACCESS_KEY_ID="your_access_key_id"
      export R2_SECRET_ACCESS_KEY="your_secret_access_key"
      ```
    - **for aws s3:**
      ```bash
      export AWS_ACCESS_KEY_ID="your_access_key_id"
      export AWS_SECRET_ACCESS_KEY="your_secret_access_key"
      export AWS_REGION="your_bucket_region" # e.g., us-east-1
      ```

4.  **update your `velocity.config.json`** with your bucket name and region.

`velocity-cache` will automatically detect these environment variables and switch to using your private bucket.

## configuration (`velocity.config.json`)

the `init` command gives you a starting point. **you must customize `inputs` and `outputs` for your project.**

```json
{
  "$schema": "[https://velocitycache.dev/schema.json](https://velocitycache.dev/schema.json)",
  "remote_cache": {
    "enabled": true,
    // required only if using a private cache
    "bucket": "your-private-bucket-name",
    "region": "auto" // e.g., us-east-1 for aws, auto for r2
  },
  "scripts": {
    "build": {
      // this key must match the script you run with 'velocity-cache run ...'
      "command": "npm run build", // the *actual* build command, change to "npm run build:real if permanent installation
      "inputs": [
        // files/globs that affect the build output
        "src/**/*",
        "public/**/*",
        "package.json",
        "package-lock.json", // include lock files!
        "tsconfig.json",
        "next.config.js"
        // add your config files: vite.config.ts, postcss.config.js, .env, etc.
      ],
      "outputs": [
        // directories created by the 'command'
        ".next/"
        // change this for other frameworks (e.g., "dist/", "build/")
      ],
      "env_keys": [
        // env vars that affect the build output
        "NODE_ENV",
        "NEXT_PUBLIC_API_URL"
        // add keys like VITE_*, CI, etc.
      ]
    }
    // you can add other scripts here, like "test", "lint", etc.
    // "test": { ... }
  }
}
```

- **`remote_cache`**: configures the remote cache.

  - `enabled`: set to `true` to use _any_ remote cache (public or private). set to `false` for local-only caching.
  - `bucket`, `region`: **only required if using a private cache.** ignored if using the public cache. `velocity-cache` uses the environment variables to authenticate.

- **`scripts`**: a map of script names you want to cache.

  - **`"build"`**: the name that maps to `package.json` (you'll run `npx velocity-cache run build`).
  - **`command`**: the _actual_ command to run on a cache miss (e.g., `npm run build:real`).
  - **`inputs`**: **critical.** list of files/globs `velocity` will hash. changes here trigger a cache miss.
  - **`outputs`**: **critical.** list of directories created by `command`. these are cached.
  - **`env_keys`**: list of environment variables included in the hash.

## how it works

`velocity-cache` creates a unique "fingerprint" (a sha256 hash) for your project state by combining:

1.  a hash of your command (e.g., `"npm run build:real"`).
2.  a hash of your relevant environment variables (e.g., `"NODE_ENV=production"`).
3.  a hash of all your `input` files' contents (respecting `.gitignore`).

this final hash (`d2...e4`) is the cache key. the `.zip` file containing your `output` directories (`.next/`) is named after this key.

## contributing

pull requests are welcome\! for major changes, please open an issue first to discuss what you would like to change.

## license

[mit](https://opensource.org/licenses/mit)
