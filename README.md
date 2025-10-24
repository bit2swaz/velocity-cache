# velocity-cache

[![build status](https://img.shields.io/badge/build-passing-brightgreen)](https://github.com/bit2swaz/velocity-cache) [![license: mit](https://img.shields.io/badge/license-mit-blue.svg)](https://opensource.org/licenses/mit)

a blazing-fast, open-source distributed build cache. built in go to accelerate your ci/cd pipelines.

---

![velocity-cache demo gif](termtosvg_mtmm_9z0.svg)

_a 1-minute next.js build (cache miss) becomes a 1-second build (cache hit)._

## why velocity-cache?

software teams waste millions of dollars and thousands of developer-hours waiting for slow builds, tests, and ci jobs. this "wait time" is a direct drain on productivity and happiness.

`velocity-cache` fixes this.

it's a simple go cli that wraps your existing build scripts (like `npm run build`). it intelligently "fingerprints" your project's state. if it's seen that exact state before—on your machine, or a teammate's, or in ci—it downloads the pre-computed result from a shared cache in seconds instead of re-running the task.

the first person pays the time cost. everyone else benefits instantly.

## features

- **blazing fast:** built in go for high-performance, concurrent hashing and caching.
- **distributed:** uses s3 (or any compatible service like r2/minio) to share a cache across your entire team and ci/cd.
- **framework-agnostic:** works with any build script or framework (next.js, vite, etc.).
- **smart:** respects your `.gitignore` and hashes file contents, not just timestamps.
- **easy to use:** a single, dependency-free binary. just run `init` and `run`.

## installation

1.  go to the [latest release](https://github.com/bit2swaz/velocity-cache/releases/latest) page.
2.  download the correct binary for your system (e.g., `velocity-cli-linux-amd64.tar.gz`).
3.  extract and move it into your path.

```bash
# for linux/macos:
tar -xvf velocity-cli-linux-amd64.tar.gz
sudo mv velocity-cli-linux-amd64 /usr/local/bin/velocity

# make sure it's executable
sudo chmod +x /usr/local/bin/velocity
```

## usage

### 1\. `velocity init`

in the root of your project, run the `init` command.

```bash
velocity init
```

this will create a `velocity.config.json` file.

### 2\. `velocity run <script-name>`

this is the main command. instead of running `npm run build`, you now run:

```bash
velocity run build
```

`velocity` will:

1.  generate a unique hash of your project.
2.  check for that hash in your local (`.velocity/cache/`) and remote (s3) cache.
3.  **if hit:** it will download the `.zip`, restore your output directories, and exit in \<1s.
4.  **if miss:** it will run your command (`npm run build`), then save the result to both caches.

### 3\. `velocity clean`

if you ever need to clear your _local_ cache:

```bash
velocity clean
```

## configuration (`velocity.config.json`)

the `init` command gives you a great starting point. here's what the fields mean:

```json
{
  "$schema": "https://velocitycache.dev/schema.json",
  "remote_cache": {
    "enabled": true,
    "bucket": "velocity-cache-mvp-public-1",
    "region": "us-east-1"
  },
  "scripts": {
    "build": {
      "command": "npm run build",
      "inputs": [
        "app/**/*",
        "src/**/*",
        "public/**/*",
        "package.json",
        "package-lock.json",
        "tsconfig.json",
        "next.config.js"
      ],
      "outputs": [".next/"],
      "env_keys": ["NODE_ENV"]
    }
  }
}
```

- **`remote_cache`**: configures the s3-compatible remote cache. this is how you share with your team.

  - `enabled`: set to `true` to use the remote cache.
  - `bucket`: the name of your s3/r2 bucket.
  - `region`: the region of your bucket (e.g., `us-east-1` for aws, `auto` for r2).

- **`scripts`**: a map of script names you want to cache.

  - **`"build"`**: the name that maps to `package.json` (you'll run `velocity run build`).
  - **`command`**: the _actual_ command to run on a cache miss (e.g., `npm run build`).
  - **`inputs`**: this is the most important part. it's a list of files and globs that `velocity` will hash. if any of these files change, it's a cache miss. _be sure to include lock files\!_
  - **`outputs`**: a list of directories that are created by your `command`. `velocity` will zip these up on a cache miss and restore them on a cache hit.
  - **`env_keys`**: a list of environment variables to include in the hash. for example, a `NODE_ENV=production` build should have a different cache key than a `NODE_ENV=development` build.

## how it works

`velocity` creates a unique "fingerprint" (a sha256 hash) for your project state by combining:

1.  a hash of your command (e.g., `"npm run build"`).
2.  a hash of your relevant environment variables (e.g., `"NODE_ENV=production"`).
3.  a hash of all your `input` files' contents (respecting `.gitignore`).

this final hash (`d2...e4`) is the cache key. the `.zip` file containing your `output` directories (`.next/`) is named after this key.

## contributing

pull requests are welcome\! for major changes, please open an issue first to discuss what you would like to change.

## license

[mit](https://opensource.org/licenses/mit)
