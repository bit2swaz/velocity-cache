#!/usr/bin/env node
const axios = require('axios');
const path = require('path');
const decompress = require('decompress');
const { rimraf } = require('rimraf');
const fs = require('fs');

const { version } = require('./package.json');

// --- Configuration ---
const VERSION = `v${version}`; // Must match the GitHub release tag
const REPO_URL = 'bit2swaz/velocity-cache';
// --- End Configuration ---

// This is the *target* name we want (e.g., 'velocity-cli')
const BIN_DIR = path.join(__dirname, 'bin');
const BIN_NAME = process.platform === 'win32' ? 'velocity-cli.exe' : 'velocity-cli';
const BIN_PATH = path.join(BIN_DIR, BIN_NAME);

/**
 * Gets the download info for the current platform.
 */
function getDownloadInfo() {
  const platform = process.platform;
  const arch = process.arch;

  let os, archSuffix;

  switch (platform) {
    case 'win32':
      os = 'windows';
      break;
    case 'linux':
      os = 'linux';
      break;
    case 'darwin':
      os = 'darwin';
      break;
    default:
      throw new Error(`Unsupported platform: ${platform}`);
  }

  switch (arch) {
    case 'x64':
      archSuffix = 'amd64';
      break;
    case 'arm64':
      archSuffix = 'arm64';
      break;
    default:
      throw new Error(`Unsupported architecture: ${arch}`);
  }

  const ext = os === 'windows' ? 'zip' : 'tar.gz';
  const archiveName = `velocity-cli-${os}-${archSuffix}.${ext}`;
  
  // This is the *source* name (the file inside the archive)
  const binaryNameInArchive = os === 'windows' 
    ? `velocity-cli-${os}-${archSuffix}.exe` 
    : `velocity-cli-${os}-${archSuffix}`;
  
  const url = `https://github.com/${REPO_URL}/releases/download/${VERSION}/${archiveName}`;
  
  return { url, binaryNameInArchive };
}

/**
 * Downloads and extracts the binary.
 */
async function install() {
  console.log('[VelocityCache] Finding correct binary for your system...');
  
  let url, binaryNameInArchive;
  try {
    const info = getDownloadInfo();
    url = info.url;
    binaryNameInArchive = info.binaryNameInArchive;
  } catch (e) {
    console.error(`[VelocityCache] Error: ${e.message}`);
    process.exit(1);
  }

  console.log(`[VelocityCache] Downloading from: ${url}`);

  try {
    // 1. Clean up old binary and bin dir
    await rimraf(BIN_DIR);
    await fs.promises.mkdir(BIN_DIR, { recursive: true });

    // 2. Download the file
    const response = await axios({
      url,
      method: 'GET',
      responseType: 'arraybuffer', // This is correct from last time
    });

    // 3. Decompress the stream
    await decompress(response.data, BIN_DIR, {
      // Filter for the *specific* binary name in the archive
      filter: file => file.path === binaryNameInArchive,
      // Map/Rename that file to the name we want
      map: file => {
        file.path = BIN_NAME; // e.g., renames 'velocity-cli-linux-amd64' to 'velocity-cli'
        return file;
      }
    });

    // 4. Make the binary executable (on non-windows)
    if (process.platform !== 'win32') {
      await fs.promises.chmod(BIN_PATH, 0o755); // rwxr-x---
    }

    console.log(`[VelocityCache] Successfully installed binary to: ${BIN_PATH}`);

  } catch (e) {
    console.error('[VelocityCache] Failed to download or install binary.');
    console.error(e); // Log the full error, not just the message
    process.exit(1);
  }
}

install();