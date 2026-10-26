#!/usr/bin/env node
/**
 * Auto-Rebuild & Auto-Deploy Watcher for Cloudflare
 * Watches frontend apps, scripts, and triggers automated builds and deployments.
 */
import { spawn } from 'node:child_process';
import { watch } from 'node:fs';
import { resolve } from 'node:path';

const WATCH_DIRS = ['apps', 'scripts'];
let building = false;
let pending = false;
let debounceTimer = null;

function runDeploy() {
  if (building) {
    pending = true;
    return;
  }
  building = true;
  console.log('\n🔄 [Auto-Rebuild] Changes detected! Rebuilding and deploying to Cloudflare...');

  const proc = spawn('npx', ['wrangler', 'deploy'], {
    shell: true,
    stdio: 'inherit'
  });

  proc.on('close', code => {
    building = false;
    if (code === 0) {
      console.log('✅ [Auto-Rebuild] Cloudflare deployment successfully updated!\n');
    } else {
      console.error(`❌ [Auto-Rebuild] Deployment exited with code ${code}\n`);
    }
    if (pending) {
      pending = false;
      runDeploy();
    }
  });
}

function trigger() {
  clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    runDeploy();
  }, 1000);
}

console.log('👀 Watching apps/ and scripts/ for automatic rebuilding & Cloudflare deployment...');
for (const dir of WATCH_DIRS) {
  try {
    watch(resolve(process.cwd(), dir), { recursive: true }, (event, filename) => {
      if (filename && !filename.includes('node_modules') && !filename.includes('.git')) {
        console.log(`📝 Change in ${dir}/${filename} (${event})`);
        trigger();
      }
    });
  } catch (err) {
    console.warn(`Could not watch ${dir}:`, err.message);
  }
}
