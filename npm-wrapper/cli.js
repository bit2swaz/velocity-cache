#!/usr/bin/env node
const path = require('path');
const { spawn } = require('child_process');

// 1. Determine the path to the binary
const platform = process.platform;
const arch = process.arch;

let binName = 'velocity-cli';
if (platform === 'win32') {
  binName += '.exe';
}

// __dirname is the current (npm-wrapper) directory
const binPath = path.join(__dirname, 'bin', binName);

// 2. Spawn the child process
// We use 'inherit' to pass through all stdin, stdout, and stderr.
// This is critical for seeing build logs, colors, etc.
const proc = spawn(binPath, process.argv.slice(2), { stdio: 'inherit' });

// 3. Pass on the exit code
proc.on('exit', (code) => {
  process.exit(code);
});

// 4. Handle errors, like if the binary wasn't downloaded
proc.on('error', (err) => {
  console.error(`[VelocityCache] Failed to start CLI: ${err.message}`);
  console.error('This likely means the postinstall script failed.');
  console.error('Please try reinstalling the package.');
  process.exit(1);
});