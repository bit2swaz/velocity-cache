#!/usr/bin/env node
const fs = require('fs');
const path = require('path');

const manifestPath = path.join(__dirname, '..', 'package.json');
const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));

function checkSection(sectionName) {
  const section = manifest[sectionName];
  if (!section) {
    return [];
  }

  return Object.entries(section)
    .filter(([_, range]) => typeof range === 'string' && range.trim().startsWith('workspace:'))
    .map(([name, range]) => ({ section: sectionName, name, range }));
}

const offenders = [
  ...checkSection('dependencies'),
  ...checkSection('devDependencies'),
  ...checkSection('optionalDependencies'),
  ...checkSection('peerDependencies'),
];

if (offenders.length === 0) {
  console.log('[velocity-cache] manifest check passed: no workspace: ranges detected.');
  process.exit(0);
}

console.error('[velocity-cache] Refusing to pack/publish with workspace: ranges in package.json');
for (const offender of offenders) {
  console.error(`  - ${offender.section}.${offender.name} = ${offender.range}`);
}
console.error('Replace these with semver ranges, tarball URLs, or published package versions before releasing.');
process.exit(1);
