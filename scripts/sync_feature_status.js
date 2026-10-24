const fs = require('fs');

const content = fs.readFileSync('docs/01_REQUIREMENTS_MATRIX_001_100.md', 'utf-8');
const lines = content.split('\n');

const rows = [];
let currPhase = '';

for (let rawLine of lines) {
  const line = rawLine.trim();
  if (line.startsWith('## Phase')) {
    currPhase = line.replace('## ', '');
    continue;
  }
  if (line.startsWith('|') && !line.startsWith('| ID') && !line.startsWith('|---')) {
    const parts = line.split('|').map(s => s.trim()).filter((_, idx, arr) => idx > 0 && idx < arr.length - 1);
    if (parts.length >= 5) {
      const num = parts[0].padStart(3, '0');
      const reqId = `R-${num}`;
      const item = parts[1];
      const desc = parts[2].replace(/"/g, '""');
      const component = parts[3].replace(/"/g, '""');
      const status = parts[4];
      rows.push(`"${reqId}","${currPhase}","${item}","${status}","${desc}","${component}"`);
    }
  }
}

const csvHeader = 'requirement_id,phase,feature,implementation_status,implemented_scope_or_gap,schema_or_proto_notes\n';
fs.writeFileSync('docs/FEATURE_STATUS_001_100.csv', csvHeader + rows.join('\n') + '\n', 'utf-8');
console.log(`Updated docs/FEATURE_STATUS_001_100.csv with ${rows.length} rows`);

