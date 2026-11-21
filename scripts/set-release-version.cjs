const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '..');
const versionPath = path.join(root, 'scripts/VERSION');
const baseVersion = fs.readFileSync(versionPath, 'utf8').trim();
const match = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.exec(baseVersion);
const runNumber = process.env.GITHUB_RUN_NUMBER;
if (!match || !/^[1-9]\d*$/.test(runNumber || '') || !process.env.GITHUB_OUTPUT) {
  throw new Error('Release version requires a major.minor.patch VERSION, GITHUB_RUN_NUMBER, and GITHUB_OUTPUT.');
}

// Each workflow run advances the patch; reruns keep their original version.
// Keep scripts/VERSION in the checkout as the baseline, without committing CI bumps.
const patch = BigInt(match[3]) + BigInt(runNumber);
const version = `${match[1]}.${match[2]}.${patch}`;
const configPath = path.join(root, 'wails.json');
const config = JSON.parse(fs.readFileSync(configPath, 'utf8'));
config.info = { ...config.info, productVersion: version };

fs.writeFileSync(versionPath, `${version}\n`);
fs.writeFileSync(configPath, `${JSON.stringify(config, null, 2)}\n`);
fs.appendFileSync(process.env.GITHUB_OUTPUT, `version=${version}\n`);
console.log(`Release version: ${version}`);
