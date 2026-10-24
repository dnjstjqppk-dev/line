const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

console.log('Building all Vue workspaces for Cloudflare deployment...');
execSync('npm run build:apps', { stdio: 'inherit' });

const distRoot = path.join(__dirname, '..', 'dist_cf');
if (fs.existsSync(distRoot)) {
  fs.rmSync(distRoot, { recursive: true, force: true });
}
fs.mkdirSync(distRoot, { recursive: true });

function copyDir(src, dest) {
  fs.mkdirSync(dest, { recursive: true });
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    const srcPath = path.join(src, entry.name);
    const destPath = path.join(dest, entry.name);
    if (entry.isDirectory()) {
      copyDir(srcPath, destPath);
    } else {
      fs.copyFileSync(srcPath, destPath);
    }
  }
}

// 1. Root app is user-web (the main Messenger application)
const userWebDist = path.join(__dirname, '..', 'apps', 'user-web', 'dist');
if (fs.existsSync(userWebDist)) {
  copyDir(userWebDist, distRoot);
}

// 2. Sub-apps accessible via /admin, /partner, /developer, /creator, /ads
const subApps = [
  { name: 'admin', path: 'admin-console' },
  { name: 'partner', path: 'partner-console' },
  { name: 'developer', path: 'developer-console' },
  { name: 'creator', path: 'creator-console' },
  { name: 'ads', path: 'ads-console' }
];

for (const sub of subApps) {
  const subSrc = path.join(__dirname, '..', 'apps', sub.path, 'dist');
  const subDest = path.join(distRoot, sub.name);
  if (fs.existsSync(subSrc)) {
    copyDir(subSrc, subDest);
  }
}

console.log('✅ Combined Cloudflare distribution prepared in ./dist_cf');
