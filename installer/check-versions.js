const fs = require('fs');
const path = require('path');

// Check CLI arguments for update flag
const shouldUpdate = process.argv.includes('--update') || process.argv.includes('-u');

// Resolve project root relative to this script (which is inside installer/)
const projectRoot = path.resolve(__dirname, '..');
const releaseFilePath = path.join(projectRoot, 'release');

// Helper for colors
const colors = {
  reset: '\x1b[0m',
  bright: '\x1b[1m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  red: '\x1b[31m',
  cyan: '\x1b[36m',
  gray: '\x1b[90m',
};

function logInfo(msg) {
  console.log(`${colors.cyan}[INFO]${colors.reset} ${msg}`);
}

function logSuccess(msg) {
  console.log(`${colors.green}[SUCCESS]${colors.reset} ${msg}`);
}

function logWarning(msg) {
  console.log(`${colors.yellow}[WARNING]${colors.reset} ${msg}`);
}

function logError(msg) {
  console.error(`${colors.red}[ERROR]${colors.reset} ${msg}`);
}

// 1. Read release version
if (!fs.existsSync(releaseFilePath)) {
  logError(`Release file not found at: ${releaseFilePath}`);
  process.exit(1);
}

const targetVersion = fs.readFileSync(releaseFilePath, 'utf8').trim();
if (!targetVersion) {
  logError(`Release file at ${releaseFilePath} is empty.`);
  process.exit(1);
}

logInfo(`Target version from release file: ${colors.bright}${targetVersion}${colors.reset}`);
if (shouldUpdate) {
  logWarning(`Update mode is ENABLED. Mismatching files will be automatically modified.\n`);
} else {
  logInfo(`Running in dry-run mode. Pass ${colors.bright}--update${colors.reset} or ${colors.bright}-u${colors.reset} to apply changes.\n`);
}

// Extract prefix to avoid matching external versions (like Windows SDK 10.0.x or Go versions)
// If targetVersion is "0.0.6.1", prefix is "0.0"
const segments = targetVersion.split('.');
if (segments.length < 2) {
  logError(`Invalid version format in release file: ${targetVersion}`);
  process.exit(1);
}
const prefix = segments.slice(0, 2).join('.');
const escapedPrefix = prefix.replace(/\./g, '\\.');

// Regex matches:
// - (?<![\d.]) -> negative lookbehind to ensure version is not part of a larger number/IP/SDK version (e.g. not preceded by digit or dot)
// - [vV]? -> optional 'v' or 'V' prefix
// - prefix -> the major/minor prefix (e.g. 0.0)
// - \.\d+(?:\.\d+)* -> dot followed by digits, optionally followed by more dot-digits
// - (?![\d.]) -> negative lookahead to ensure it doesn't end with a dot or digits (part of a larger version)
const versionRegex = new RegExp(`(?<![\\d.])[vV]?${escapedPrefix}\\.\\d+(?:\\.\\d+)*(?![\\d.])`, 'g');

logInfo(`Scanning files for version pattern matching prefix "${prefix}"...\n`);

// Excluded directories and file extensions
const excludedDirs = new Set([
  '.git',
  '.github',
  '.kiro',
  '.vscode',
  'node_modules',
  'build',
  'assets',
  'images',
  'store-assets'
]);

const excludedExtensions = new Set([
  '.exe',
  '.msi',
  '.msix',
  '.msixupload',
  '.syso',
  '.png',
  '.jpg',
  '.jpeg',
  '.gif',
  '.ico',
  '.pfx',
  '.zip',
  '.gz',
  '.tgz',
  '.mod', // Exclude go.mod
  '.sum'  // Exclude go.sum
]);

let totalFilesScanned = 0;
let totalMatches = 0;
let totalMismatches = 0;
let totalFilesUpdated = 0;

function shouldScanFile(filePath, fileName) {
  const ext = path.extname(fileName).toLowerCase();
  if (excludedExtensions.has(ext)) return false;
  if (fileName === 'go.mod' || fileName === 'go.sum') return false;
  
  // Skip binary/large files or this script itself
  if (fileName === 'check-versions.js') return false;
  
  return true;
}

function scanDir(dirPath) {
  const items = fs.readdirSync(dirPath, { withFileTypes: true });
  
  for (const item of items) {
    const fullPath = path.join(dirPath, item.name);
    
    if (item.isDirectory()) {
      if (excludedDirs.has(item.name)) continue;
      scanDir(fullPath);
    } else if (item.isFile()) {
      if (shouldScanFile(fullPath, item.name)) {
        checkFile(fullPath);
      }
    }
  }
}

function checkFile(filePath) {
  totalFilesScanned++;
  let content;
  try {
    // Read file. If it contains null bytes, it might be binary; skip it.
    const buffer = fs.readFileSync(filePath);
    if (buffer.includes(0)) {
      return; // Skip binary files
    }
    content = buffer.toString('utf8');
  } catch (err) {
    // Ignore read errors
    return;
  }
  
  const lines = content.split(/\r?\n/);
  const relativePath = path.relative(projectRoot, filePath);
  let filePrinted = false;
  let fileModified = false;
  
  const updatedLines = lines.map((line, lineIndex) => {
    // Reset regex index for safety
    versionRegex.lastIndex = 0;
    
    let match;
    let lineMatches = [];
    
    while ((match = versionRegex.exec(line)) !== null) {
      const foundFull = match[0];
      // Strip 'v' or 'V' prefix to get the raw version number
      const foundVersion = foundFull.replace(/^[vV]/, '');
      
      const isMatch = (foundVersion === targetVersion);
      lineMatches.push({
        foundFull,
        foundVersion,
        isMatch,
        index: match.index
      });
    }
    
    if (lineMatches.length > 0) {
      if (!filePrinted) {
        console.log(`${colors.bright}${colors.cyan}./${relativePath.replace(/\\/g, '/')}${colors.reset}`);
        filePrinted = true;
      }
      
      // We want to highlight the match on the line.
      let formattedLine = '';
      let lastIndex = 0;
      
      lineMatches.forEach(m => {
        // Add part before match
        formattedLine += line.substring(lastIndex, m.index);
        
        // Add highlighted match
        if (m.isMatch) {
          formattedLine += `${colors.green}${m.foundFull}${colors.reset}`;
          totalMatches++;
        } else {
          formattedLine += `${colors.red}${colors.bright}${m.foundFull}${colors.reset}`;
          totalMismatches++;
          fileModified = true;
        }
        
        lastIndex = m.index + m.foundFull.length;
      });
      
      formattedLine += line.substring(lastIndex);
      
      // Print line info
      const lineNumberStr = String(lineIndex + 1).padStart(4, ' ');
      console.log(`  ${colors.gray}${lineNumberStr}:${colors.reset} ${formattedLine.trim()}`);
      
      if (shouldUpdate && fileModified) {
        // Replace all mismatching occurrences on the line
        versionRegex.lastIndex = 0;
        return line.replace(versionRegex, (m) => {
          const hasV = m.match(/^[vV]/);
          return (hasV ? hasV[0] : '') + targetVersion;
        });
      }
    }
    
    return line;
  });
  
  if (shouldUpdate && fileModified) {
    try {
      const lineEnding = content.includes('\r\n') ? '\r\n' : '\n';
      fs.writeFileSync(filePath, updatedLines.join(lineEnding), 'utf8');
      console.log(`  ${colors.green}${colors.bright}[UPDATED]${colors.reset} Saved changes to ./${relativePath.replace(/\\/g, '/')}`);
      totalFilesUpdated++;
    } catch (err) {
      logError(`Failed to write updates to ./${relativePath.replace(/\\/g, '/')}: ${err.message}`);
    }
  }
}

// Start scanning
scanDir(projectRoot);

console.log('\n' + '='.repeat(60));
console.log(`${colors.bright}Scan Summary:${colors.reset}`);
console.log(`  Total files scanned:      ${totalFilesScanned}`);
console.log(`  Matching versions:        ${colors.green}${totalMatches}${colors.reset}`);
console.log(`  Mismatching versions:     ${totalMismatches > 0 ? colors.red + colors.bright : colors.gray}${totalMismatches}${colors.reset}`);
if (shouldUpdate) {
  console.log(`  Total files updated:      ${colors.green}${totalFilesUpdated}${colors.reset}`);
}
console.log('='.repeat(60));

if (totalMismatches > 0) {
  if (shouldUpdate) {
    logSuccess(`Successfully updated all ${totalMismatches} mismatches across ${totalFilesUpdated} files.`);
  } else {
    logWarning(`Found ${totalMismatches} version strings that do not match the target version (${targetVersion}). Run with --update to fix them.`);
  }
} else {
  logSuccess(`All version strings match the target version (${targetVersion})!`);
}
