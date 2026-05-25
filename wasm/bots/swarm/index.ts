// SwarmBot WASM implementation - formation-based combat with tight cohesion.
// Uses low-level WASM interface compatible with the sandbox loader.

// Configuration stored globally
let rows: i32 = 60;
let cols: i32 = 60;
let attackRadius2: i32 = 12;
let visionRadius2: i32 = 16;

// Bot ID
let myId: i32 = 0;

// Swarm configuration
const COHESION_RADIUS: i32 = 3;
const COHESION_RADIUS2: i32 = 9;

// Dynamic arrays for game state
let myBots: Position[] = [];
let enemyBots: Position[] = [];
let walls: Position[] = [];
let energy: Position[] = [];

// Position structure
class Position {
  row: i32 = 0;
  col: i32 = 0;

  constructor(row: i32 = 0, col: i32 = 0) {
    this.row = row;
    this.col = col;
  }

  equals(other: Position): bool {
    return this.row == other.row && this.col == other.col;
  }

  toString(): string {
    return `${this.row},${this.col}`;
  }
}

// Direction enum
const DIR_N: i32 = 0;
const DIR_E: i32 = 1;
const DIR_S: i32 = 2;
const DIR_W: i32 = 3;

const DROW: i32[] = [-1, 0, 1, 0];
const DCOL: i32[] = [0, 1, 0, -1];
const DIR_NAMES: string[] = ["N", "E", "S", "W"];

// Move output structure
class Move {
  position: Position;
  direction: i32;

  constructor(pos: Position, dir: i32) {
    this.position = pos;
    this.direction = dir;
  }
}

// Output buffer for moves
let outputMoves: Move[] = [];

// Initialize the bot with game config
export function init(configJson: string): string {
  // Parse config JSON: {"rows":60,"cols":60,"attack_radius2":12,"vision_radius2":16}
  // Simple manual parsing for performance
  let ptr = 0;
  let len = configJson.length;

  while (ptr < len) {
    // Skip whitespace
    while (ptr < len && configJson.charCodeAt(ptr) <= 32) ptr++;
    if (ptr >= len) break;

    // Find key
    if (configJson.charCodeAt(ptr) == 34) { // "
      ptr++;
      let keyStart = ptr;
      while (ptr < len && configJson.charCodeAt(ptr) != 34) ptr++;
      let key = configJson.substring(keyStart, ptr);
      ptr++; // skip closing "

      // Skip to colon
      while (ptr < len && configJson.charCodeAt(ptr) != 58) ptr++;
      ptr++; // skip colon

      // Parse value
      while (ptr < len && configJson.charCodeAt(ptr) <= 32) ptr++;
      let valueStart = ptr;
      let neg = false;
      if (configJson.charCodeAt(ptr) == 45) { // -
        neg = true;
        ptr++;
      }
      while (ptr < len && configJson.charCodeAt(ptr) >= 48 && configJson.charCodeAt(ptr) <= 57) ptr++;
      let valStr = configJson.substring(valueStart, ptr);
      let val = parseInt(valStr);
      let value: i64 = neg ? -i64(val) : i64(val);

      // Set config
      if (key == "rows") rows = i32(value);
      else if (key == "cols") cols = i32(value);
      else if (key == "attack_radius2") attackRadius2 = i32(value);
      else if (key == "vision_radius2") visionRadius2 = i32(value);
    } else {
      ptr++;
    }
  }

  return "{\"ok\":true}";
}

// Parse JSON game state and compute moves
export function compute_moves(stateJson: string): string {
  // Clear previous state
  myBots = [];
  enemyBots = [];
  walls = [];
  energy = [];
  outputMoves = [];

  // Parse state JSON
  parseGameState(stateJson);

  // Compute moves using swarm strategy
  computeSwarmMoves();

  // Build output JSON
  return buildOutputJSON();
}

// Parse game state from JSON
function parseGameState(json: string): void {
  let ptr = 0;
  let len = json.length;
  let inString = false;
  let inArray = false;
  let inObject = false;
  let currentKey = "";
  let currentValue = "";
  let parsingKey = true;
  let depth = 0;
  let arrayDepth = 0;
  let targetArray: string = "";
  let inBotsArray = false;
  let inWallsArray = false;
  let inEnergyArray = false;

  while (ptr < len) {
    let ch = json.charCodeAt(ptr);

    if (ch == 34) { // "
      inString = !inString;
    } else if (!inString) {
      if (ch == 123) { // {
        inObject = true;
        depth++;
        if (depth == 2) parsingKey = true;
      } else if (ch == 125) { // }
        inObject = false;
        depth--;
        if (depth == 1) parsingKey = true;
      } else if (ch == 91) { // [
        inArray = true;
        arrayDepth++;
        if (currentKey == "bots" && arrayDepth == 2) {
          inBotsArray = true;
        } else if (currentKey == "walls" && arrayDepth == 2) {
          inWallsArray = true;
        } else if (currentKey == "energy" && arrayDepth == 2) {
          inEnergyArray = true;
        }
      } else if (ch == 93) { // ]
        inArray = false;
        arrayDepth--;
        inBotsArray = false;
        inWallsArray = false;
        inEnergyArray = false;
      } else if (ch == 58) { // :
        parsingKey = false;
      } else if (ch == 44) { // ,
        parsingKey = true;
        if (depth == 1) {
          currentKey = "";
          currentValue = "";
        }
      } else if (parsingKey && ch > 32) {
        let start = ptr;
        while (ptr < len && json.charCodeAt(ptr) > 32 && json.charCodeAt(ptr) != 58) ptr++;
        currentKey = json.substring(start, ptr);
        continue;
      }
    }

    ptr++;
  }

  // Extract key values using string search (simplified approach)
  extractMyId(json);
  extractBotPositions(json);
  extractWallPositions(json);
  extractEnergyPositions(json);
}

// Extract my ID from JSON
function extractMyId(json: string): void {
  let key = "\"you\":";
  let idx = json.indexOf(key);
  if (idx >= 0) {
    idx += key.length;
    let idKey = "\"id\":";
    let idIdx = json.indexOf(idKey, idx);
    if (idIdx >= 0) {
      idIdx += idKey.length;
      let end = idIdx;
      while (end < json.length && json.charCodeAt(end) >= 48 && json.charCodeAt(end) <= 57) end++;
      myId = i32(parseInt(json.substring(idIdx, end)));
    }
  }
}

// Extract bot positions from JSON
function extractBotPositions(json: string): void {
  let botsStart = json.indexOf("\"bots\":[");
  if (botsStart < 0) return;

  botsStart += 8; // Skip "bots":[
  let depth = 1;
  let pos = botsStart;

  while (pos < json.length && depth > 0) {
    let ch = json.charCodeAt(pos);

    if (ch == 123) { // { - start of bot object
      let botEnd = json.indexOf("}", pos);
      if (botEnd < 0) break;

      let botJson = json.substring(pos, botEnd + 1);

      // Extract position
      let posStart = botJson.indexOf("\"position\":");
      if (posStart >= 0) {
        posStart = botJson.indexOf("{", posStart);
        let posEnd = botJson.indexOf("}", posStart);
        if (posEnd > posStart) {
          let posJson = botJson.substring(posStart, posEnd + 1);

          let rowStart = posJson.indexOf("\"row\":");
          let colStart = posJson.indexOf("\"col\":");

          if (rowStart >= 0 && colStart >= 0) {
            rowStart += 6;
            colStart += 6;

            let rowEnd = rowStart;
            while (rowEnd < posJson.length && (posJson.charCodeAt(rowEnd) >= 48 && posJson.charCodeAt(rowEnd) <= 57 || posJson.charCodeAt(rowEnd) == 45)) rowEnd++;

            let colEnd = colStart;
            while (colEnd < posJson.length && (posJson.charCodeAt(colEnd) >= 48 && posJson.charCodeAt(colEnd) <= 57 || posJson.charCodeAt(colEnd) == 45)) colEnd++;

            let row = i32(parseInt(posJson.substring(rowStart, rowEnd)));
            let col = i32(parseInt(posJson.substring(colStart, colEnd)));

            // Extract owner
            let ownerStart = botJson.indexOf("\"owner\":");
            if (ownerStart >= 0) {
              ownerStart += 9;
              let ownerEnd = ownerStart;
              while (ownerEnd < botJson.length && (botJson.charCodeAt(ownerEnd) >= 48 && botJson.charCodeAt(ownerEnd) <= 57)) ownerEnd++;
              let owner = i32(parseInt(botJson.substring(ownerStart, ownerEnd)));

              let botPos = new Position(row, col);
              if (owner == myId) {
                myBots.push(botPos);
              } else {
                enemyBots.push(botPos);
              }
            }
          }
        }
      }

      pos = botEnd + 1;
    } else if (ch == 91) {
      depth++;
      pos++;
    } else if (ch == 93) {
      depth--;
      pos++;
    } else {
      pos++;
    }
  }
}

// Extract wall positions from JSON
function extractWallPositions(json: string): void {
  let wallsStart = json.indexOf("\"walls\":[");
  if (wallsStart < 0) return;

  wallsStart += 10; // Skip "walls":[
  let depth = 1;
  let pos = wallsStart;

  while (pos < json.length && depth > 0) {
    let ch = json.charCodeAt(pos);

    if (ch == 123) { // { - start of wall object
      let wallEnd = json.indexOf("}", pos);
      if (wallEnd < 0) break;

      let wallJson = json.substring(pos, wallEnd + 1);

      let rowStart = wallJson.indexOf("\"row\":");
      let colStart = wallJson.indexOf("\"col\":");

      if (rowStart >= 0 && colStart >= 0) {
        rowStart += 6;
        colStart += 6;

        let rowEnd = rowStart;
        while (rowEnd < wallJson.length && (wallJson.charCodeAt(rowEnd) >= 48 && wallJson.charCodeAt(rowEnd) <= 57 || wallJson.charCodeAt(rowEnd) == 45)) rowEnd++;

        let colEnd = colStart;
        while (colEnd < wallJson.length && (wallJson.charCodeAt(colEnd) >= 48 && wallJson.charCodeAt(colEnd) <= 57 || wallJson.charCodeAt(colEnd) == 45)) colEnd++;

        let row = i32(parseInt(wallJson.substring(rowStart, rowEnd)));
        let col = i32(parseInt(wallJson.substring(colStart, colEnd)));

        walls.push(new Position(row, col));
      }

      pos = wallEnd + 1;
    } else if (ch == 91) {
      depth++;
      pos++;
    } else if (ch == 93) {
      depth--;
      pos++;
    } else {
      pos++;
    }
  }
}

// Extract energy positions from JSON
function extractEnergyPositions(json: string): void {
  let energyStart = json.indexOf("\"energy\":[");
  if (energyStart < 0) return;

  energyStart += 10; // Skip "energy":[
  let depth = 1;
  let pos = energyStart;

  while (pos < json.length && depth > 0) {
    let ch = json.charCodeAt(pos);

    if (ch == 123) { // { - start of energy object
      let energyEnd = json.indexOf("}", pos);
      if (energyEnd < 0) break;

      let energyJson = json.substring(pos, energyEnd + 1);

      let rowStart = energyJson.indexOf("\"row\":");
      let colStart = energyJson.indexOf("\"col\":");

      if (rowStart >= 0 && colStart >= 0) {
        rowStart += 6;
        colStart += 6;

        let rowEnd = rowStart;
        while (rowEnd < energyJson.length && (energyJson.charCodeAt(rowEnd) >= 48 && energyJson.charCodeAt(rowEnd) <= 57 || energyJson.charCodeAt(rowEnd) == 45)) rowEnd++;

        let colEnd = colStart;
        while (colEnd < energyJson.length && (energyJson.charCodeAt(colEnd) >= 48 && energyJson.charCodeAt(colEnd) <= 57 || energyJson.charCodeAt(colEnd) == 45)) colEnd++;

        let row = i32(parseInt(energyJson.substring(rowStart, rowEnd)));
        let col = i32(parseInt(energyJson.substring(colStart, colEnd)));

        energy.push(new Position(row, col));
      }

      pos = energyEnd + 1;
    } else if (ch == 91) {
      depth++;
      pos++;
    } else if (ch == 93) {
      depth--;
      pos++;
    } else {
      pos++;
    }
  }
}

// Compute swarm moves
function computeSwarmMoves(): void {
  if (myBots.length == 0) return;

  // Calculate swarm center
  let swarmCenter = calculateCenter(myBots);

  // Calculate enemy center
  let enemyCenter: Position | null = null;
  if (enemyBots.length > 0) {
    enemyCenter = calculateCenter(enemyBots);
  }

  // Target is enemy center or map center
  let target = enemyCenter != null ? enemyCenter : new Position(rows / 2, cols / 2);

  // Compute move for each bot
  for (let i = 0; i < myBots.length; i++) {
    let bot = myBots[i];
    let move = computeBotMove(bot, swarmCenter, target);

    if (move != null) {
      outputMoves.push(move);
    }
  }
}

// Calculate center of mass using circular mean for toroidal coordinates
function calculateCenter(positions: Position[]): Position {
  if (positions.length == 0) {
    return new Position(rows / 2, cols / 2);
  }

  let sumSinRow = 0.0;
  let sumCosRow = 0.0;
  let sumSinCol = 0.0;
  let sumCosCol = 0.0;

  let rowScale = (2.0 * 3.14159265359) / f64(rows);
  let colScale = (2.0 * 3.14159265359) / f64(cols);

  for (let i = 0; i < positions.length; i++) {
    let pos = positions[i];
    sumSinRow += Math.sin(f64(pos.row) * rowScale);
    sumCosRow += Math.cos(f64(pos.row) * rowScale);
    sumSinCol += Math.sin(f64(pos.col) * colScale);
    sumCosCol += Math.cos(f64(pos.col) * colScale);
  }

  let avgRow = Math.atan2(sumSinRow / f64(positions.length), sumCosRow / f64(positions.length)) / rowScale;
  let avgCol = Math.atan2(sumSinCol / f64(positions.length), sumCosCol / f64(positions.length)) / colScale;

  let resultRow = i32(((avgRow % f64(rows)) + f64(rows)) % f64(rows));
  let resultCol = i32(((avgCol % f64(cols)) + f64(cols)) % f64(cols));

  return new Position(resultRow, resultCol);
}

// Compute move for a single bot
function computeBotMove(bot: Position, swarmCenter: Position, target: Position): Move | null {
  let bestDir: i32 = -1;
  let bestScore: f64 = -Infinity;

  for (let dir = 0; dir < 4; dir++) {
    let newPos = moveInDirection(bot, dir);

    // Check if position is valid
    if (!isValidPosition(newPos)) continue;

    // Check cohesion
    if (!maintainsCohesion(newPos, bot)) continue;

    // Score this move
    let score: f64 = 0;

    // Reward moving toward target
    let distToTarget = distance2(newPos, target);
    let currentDistToTarget = distance2(bot, target);
    score += (f64(currentDistToTarget) - f64(distToTarget)) * 10.0;

    // Penalize being far from swarm center
    let distToSwarm = distance2(newPos, swarmCenter);
    score -= f64(distToSwarm) * 0.5;

    // Bonus for moving toward enemies
    let nearestEnemyDist: i32 = 999999;
    for (let i = 0; i < enemyBots.length; i++) {
      let dist = distance2(newPos, enemyBots[i]);
      if (dist < nearestEnemyDist) {
        nearestEnemyDist = dist;
      }
    }

    if (nearestEnemyDist < 999999) {
      if (nearestEnemyDist <= attackRadius2) {
        score += 50.0;
      }
    }

    if (score > bestScore) {
      bestScore = score;
      bestDir = dir;
    }
  }

  if (bestDir >= 0) {
    return new Move(bot, bestDir);
  }

  return null;
}

// Move in a direction (with toroidal wrap)
function moveInDirection(pos: Position, dir: i32): Position {
  let newRow = pos.row + DROW[dir];
  let newCol = pos.col + DCOL[dir];

  // Toroidal wrap
  if (newRow < 0) newRow = rows - 1;
  if (newRow >= rows) newRow = 0;
  if (newCol < 0) newCol = cols - 1;
  if (newCol >= cols) newCol = 0;

  return new Position(newRow, newCol);
}

// Check if position is valid (not wall, not enemy)
function isValidPosition(pos: Position): bool {
  // Check walls
  for (let i = 0; i < walls.length; i++) {
    if (walls[i].equals(pos)) return false;
  }

  // Check enemy bots
  for (let i = 0; i < enemyBots.length; i++) {
    if (enemyBots[i].equals(pos)) return false;
  }

  return true;
}

// Check if move maintains cohesion
function maintainsCohesion(newPos: Position, oldPos: Position): bool {
  for (let i = 0; i < myBots.length; i++) {
    let other = myBots[i];
    if (other.equals(oldPos)) continue;

    let dist2 = distance2(newPos, other);
    if (dist2 <= COHESION_RADIUS2) {
      return true;
    }
  }

  // Single bot case: always allow movement
  if (myBots.length == 1) {
    return true;
  }

  return false;
}

// Squared distance (toroidal)
function distance2(a: Position, b: Position): i32 {
  let dr = Math.abs(a.row - b.row);
  let dc = Math.abs(a.col - b.col);

  if (dr > rows / 2) dr = rows - dr;
  if (dc > cols / 2) dc = cols - dc;

  return i32(dr * dr + dc * dc);
}

// Build output JSON
function buildOutputJSON(): string {
  if (outputMoves.length == 0) {
    return "{\"moves\":[]}";
  }

  let result = "{\"moves\":[";
  for (let i = 0; i < outputMoves.length; i++) {
    let move = outputMoves[i];
    result += "{\"position\":{\"row\":";
    result += move.position.row.toString();
    result += ",\"col\":";
    result += move.position.col.toString();
    result += "},\"direction\":\"";
    result += DIR_NAMES[move.direction];
    result += "\"}";

    if (i < outputMoves.length - 1) {
      result += ",";
    }
  }
  result += "]}";

  return result;
}

// Free result is a no-op for AssemblyScript (GC handles memory)
export function free_result(ptr: usize): void {
  // GC handles memory automatically
}
