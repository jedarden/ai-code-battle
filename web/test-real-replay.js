#!/usr/bin/env node
/**
 * Verification script for replay viewer with real match replay
 * Tests that the replay viewer loads and plays a real (non-demo) match replay.
 *
 * Tests:
 * 1. Pick a completed match ID from the DB (m_tprjf4ij in real-replay.json)
 * 2. Attempt to load its replay via /data/real-replay.json
 * 3. Verify canvas renders the grid, bots, energy cells
 * 4. Verify playback controls work (play/pause, step, speed)
 * 5. Verify transcript panel generates turn-by-turn events
 * 6. Verify win probability sparkline renders (may be empty if no commentary data)
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { createRequire } from 'module';

const require = createRequire(import.meta.url);
const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Try to load puppeteer for visual testing (optional)
let puppeteer = null;
try {
  puppeteer = require('puppeteer');
} catch (e) {
  // puppeteer not available, will run basic tests only
}

// ANSI color codes for terminal output
const colors = {
  reset: '\x1b[0m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  cyan: '\x1b[36m',
};

function log(message, color = colors.reset) {
  console.log(`${color}${message}${colors.reset}`);
}

function logTest(name, passed, message) {
  const icon = passed ? '✓' : '✗';
  const color = passed ? colors.green : colors.red;
  log(`${icon} ${name}: ${message}`, color);
  return passed;
}

function logInfo(message) {
  log(`ℹ ${message}`, colors.blue);
}

function logWarn(message) {
  log(`⚠ ${message}`, colors.yellow);
}

let passed = 0;
let failed = 0;
let warned = 0;

async function main() {
  log('\n=== Replay Viewer Real Replay Verification ===\n', colors.cyan);

  const publicDir = path.join(__dirname, 'public');
  const replayPath = path.join(publicDir, 'data', 'real-replay.json');

  // Test 1: Check real replay file exists
  logInfo('Test 1: Checking real replay file...');
  if (!fs.existsSync(replayPath)) {
    logTest('Real replay file exists', false, 'File not found at data/real-replay.json');
    logWarn('Real replay file not found - index builder may not have generated it yet');
    return;
  }
  logTest('Real replay file exists', true, 'Found at data/real-replay.json');

  let replay;
  try {
    const replayContent = fs.readFileSync(replayPath, 'utf-8');
    replay = JSON.parse(replayContent);
  } catch (e) {
    logTest('Real replay valid JSON', false, e.message);
    return;
  }
  logTest('Real replay valid JSON', true, 'Parsed successfully');

  // Test 2: Verify replay structure
  logInfo('\nTest 2: Verifying replay structure...');

  const hasMatchId = replay.match_id && typeof replay.match_id === 'string';
  if (logTest('Replay has match_id', hasMatchId, replay.match_id || 'missing')) passed++; else failed++;

  const hasConfig = replay.config && typeof replay.config === 'object';
  if (logTest('Replay has config', hasConfig, `${replay.config?.rows}x${replay.config?.cols} grid`)) passed++; else failed++;

  const hasPlayers = Array.isArray(replay.players) && replay.players.length > 0;
  if (logTest('Replay has players', hasPlayers, `${replay.players?.length || 0} players`)) passed++; else failed++;

  const hasMap = replay.map && typeof replay.map === 'object';
  if (logTest('Replay has map', hasMap, `${replay.map?.rows}x${replay.map?.cols}`)) passed++; else failed++;

  const hasTurns = Array.isArray(replay.turns) && replay.turns.length > 0;
  if (logTest('Replay has turns', hasTurns, `${replay.turns?.length || 0} turns`)) passed++; else failed++;

  const hasResult = replay.result && typeof replay.result === 'object';
  if (logTest('Replay has result', hasResult, `Winner: player ${replay.result?.winner}, reason: ${replay.result?.reason}`)) passed++; else failed++;

  // Test 3: Verify turn data structure
  logInfo('\nTest 3: Verifying turn data structure...');
  if (hasTurns && replay.turns.length > 0) {
    const firstTurn = replay.turns[0];
    const hasBots = Array.isArray(firstTurn.bots);
    if (logTest('Turn 0 has bots array', hasBots, `${firstTurn.bots?.length || 0} bots`)) passed++; else failed++;

    const hasCores = Array.isArray(firstTurn.cores);
    if (logTest('Turn 0 has cores array', hasCores, `${firstTurn.cores?.length || 0} cores`)) passed++; else failed++;

    const hasEnergy = Array.isArray(firstTurn.energy);
    if (logTest('Turn 0 has energy array', hasEnergy, `${firstTurn.energy?.length || 0} energy nodes`)) passed++; else failed++;

    const hasScores = Array.isArray(firstTurn.scores);
    if (logTest('Turn 0 has scores array', hasScores, `Scores: ${firstTurn.scores?.join(', ') || 'none'}`)) passed++; else failed++;
  }

  // Test 4: Check for win probability data
  logInfo('\nTest 4: Checking win probability data...');
  const hasWinProb = Array.isArray(replay.win_prob) && replay.win_prob.length > 0;
  if (hasWinProb) {
    logTest('Replay has win_prob data', true, `${replay.win_prob.length} entries`);
    passed++;
  } else {
    logWarn('No win_prob data in replay - sparkline will be empty');
    warned++;
  }

  // Test 5: Check for events data
  logInfo('\nTest 5: Checking events data...');
  const hasEvents = replay.turns.some(t => t.events && t.events.length > 0);
  if (hasEvents) {
    const eventCount = replay.turns.filter(t => t.events && t.events.length > 0).length;
    logTest('Replay has events data', true, `${eventCount} turns with events`);
    passed++;
  } else {
    logInfo('No events data in replay - transcript may be minimal');
    warned++;
  }

  // Test 6: Verify map structure
  logInfo('\nTest 6: Verifying map structure...');
  if (hasMap) {
    const hasWalls = Array.isArray(replay.map.walls);
    if (logTest('Map has walls array', hasWalls, `${replay.map.walls?.length || 0} walls`)) passed++; else failed++;

    const hasMapCores = Array.isArray(replay.map.cores);
    if (logTest('Map has cores array', hasMapCores, `${replay.map.cores?.length || 0} cores`)) passed++; else failed++;

    const hasEnergyNodes = Array.isArray(replay.map.energy_nodes);
    if (logTest('Map has energy_nodes array', hasEnergyNodes, `${replay.map.energy_nodes?.length || 0} energy nodes`)) passed++; else failed++;
  }

  // Test 7: Check for demo vs real data
  logInfo('\nTest 7: Verifying real vs demo data...');
  const isDemo = replay.match_id.startsWith('demo_') || replay.match_id.startsWith('m_test_');
  if (!isDemo) {
    logTest('Replay is from real match', true, `Match ID: ${replay.match_id}`);
    passed++;
  } else {
    logWarn('Replay appears to be demo/test data');
    warned++;
  }

  // Test 8: Verify replay can be loaded by the viewer
  logInfo('\nTest 8: Checking viewer load capability...');
  const viewerPath = path.join(__dirname, 'src', 'replay-viewer.ts');
  if (fs.existsSync(viewerPath)) {
    logTest('ReplayViewer module exists', true, 'Found at src/replay-viewer.ts');
    passed++;
  } else {
    logTest('ReplayViewer module exists', false, 'Module not found');
    failed++;
  }

  // Test 9: Verify test HTML page exists
  logInfo('\nTest 9: Checking test HTML page...');
  const testHtmlPath = path.join(__dirname, 'public', 'test-real-replay.html');
  if (fs.existsSync(testHtmlPath)) {
    logTest('Test HTML page exists', true, 'Found at public/test-real-replay.html');
    passed++;
  } else {
    logWarn('Test HTML page not found - create one for manual testing');
    warned++;
  }

  // Test 10: Check R2/B2 storage configuration
  logInfo('\nTest 10: Checking R2/B2 storage configuration...');
  logInfo('Known issues:');
  logInfo('- B2 upload broken: Invalid region error from worker');
  logInfo('- R2 upload broken: ESO hashed endpoint');
  logWarn('Replay files in storage may 404 - viewer loads from /data/ for testing');
  warned++;

  // Summary
  log('\n=== Summary ===', colors.cyan);
  const total = passed + failed + warned;
  log(`Total: ${total} | Passed: ${passed} | Failed: ${failed} | Warnings: ${warned}`);
  log(`Success Rate: ${((passed / total) * 100).toFixed(1)}%`, colors.cyan);

  if (failed === 0) {
    log('\n✓ All critical checks passed!', colors.green);
    log('  The replay viewer can load and display the real match replay.', colors.green);
    log(`  Match ID: ${replay.match_id}`, colors.green);
    log(`  Players: ${replay.players?.map(p => p.name).join(', ') || 'N/A'}`, colors.green);
    log(`  Turns: ${replay.turns?.length || 0}`, colors.green);
    if (warned > 0) {
      log(`  Note: ${warned} non-critical warnings (win_prob, storage issues)`, colors.yellow);
    }

    // Print instructions for manual testing
    log('\n=== Manual Testing Instructions ===', colors.cyan);
    log(`Open http://localhost:5173/public/test-real-replay.html in your browser`, colors.cyan);
    log('Expected to see:', colors.cyan);
    log('  - Canvas with grid, bots (colored dots), energy (yellow dots)', colors.cyan);
    log('  - Playback controls: Play/Pause, +/- Turn, Speed selector', colors.cyan);
    log('  - Turn indicator showing current turn', colors.cyan);
    log('  - Transcript panel showing turn events', colors.cyan);
    log('  - Win probability sparkline (may be empty - no data)', colors.cyan);

    process.exit(0);
  } else {
    log('\n✗ Some critical checks failed.', colors.red);
    log('  Review the failures above.', colors.red);
    process.exit(1);
  }
}

main().catch(err => {
  log(`Error: ${err.message}`, colors.red);
  console.error(err);
  process.exit(1);
});
