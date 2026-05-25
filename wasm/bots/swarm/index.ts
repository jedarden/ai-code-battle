// AssemblyScript implementation of SwarmBot for WASM compilation.
// SwarmBot keeps units in tight formations and advances as a group.

@external("env", "memory")
declare function memory: WebAssembly.Memory;

export namespace swarmBot {
  // Configuration stored globally
  let rows: i32 = 60;
  let cols: i32 = 60;
  let attackRadius2: i32 = 12;

  // Visible state
  let myId: i32 = 0;
  let botPositions: Array<f64> = new Array<f64>();
  let botOwners: Array<i32> = new Array<i32>();

  // Initialize the bot with game config
  export function init(configJson: string): string {
    try {
      const config = JSON.parse(configJson);
      if (config.rows) rows = config.rows;
      if (config.cols) cols = config.cols;
      if (config.attack_radius2) attackRadius2 = config.attack_radius2;
      return JSON.stringify({ ok: true });
    } catch (e) {
      return JSON.stringify({ ok: false, error: "parse error" });
    }
  }

  // Compute moves for the current turn
  export function compute_moves(stateJson: string): string {
    try {
      const state = JSON.parse(stateJson);
      myId = state.you.id;

      // Parse bots
      botPositions = new Array<f64>();
      botOwners = new Array<i32>();
      if (state.bots instanceof Array) {
        for (let i = 0; i < state.bots.length; i++) {
          const bot = state.bots[i];
          botPositions.push(<f64>(bot.position.row * 1000 + bot.position.col));
          botOwners.push(bot.owner);
        }
      }

      const moves = new Array<any>();
      for (let i = 0; i < state.bots.length; i++) {
        const bot = state.bots[i];
        if (bot.owner !== myId) continue;

        const dir = computeSwarmDir(bot.position.row, bot.position.col, state.bots);
        moves.push({
          position: { row: bot.position.row, col: bot.position.col },
          direction: dir
        });
      }

      return JSON.stringify(moves);
    } catch (e) {
      return "[]";
    }
  }

  // Free result is a no-op for AssemblyScript
  export function free_result(ptr: usize): void {
    // GC handles memory
  }

  // Compute swarm direction: move to maximize distance from friendly bots
  function computeSwarmDir(row: i32, col: i32, allBots: any[]): string {
    const dirs = ["N", "E", "S", "W"];
    let bestDir = "N";
    let bestScore = -1;

    for (let i = 0; i < dirs.length; i++) {
      const dir = dirs[i];
      const nr = wrapRow(row + deltaRow(dir));
      const nc = wrapCol(col + deltaCol(dir));
      let score = 0;

      for (let j = 0; j < allBots.length; j++) {
        const other = allBots[j];
        if (other.owner === myId) {
          score += dist2(nr, nc, other.position.row, other.position.col);
        }
      }

      if (score > bestScore) {
        bestScore = score;
        bestDir = dir;
      }
    }

    return bestDir;
  }

  function deltaRow(dir: string): i32 {
    switch (dir) {
      case "N": return -1;
      case "S": return 1;
      default: return 0;
    }
  }

  function deltaCol(dir: string): i32 {
    switch (dir) {
      case "E": return 1;
      case "W": return -1;
      default: return 0;
    }
  }

  function wrapRow(r: i32): i32 {
    r = r % rows;
    if (r < 0) r += rows;
    return r;
  }

  function wrapCol(c: i32): i32 {
    c = c % cols;
    if (c < 0) c += cols;
    return c;
  }

  function dist2(r1: i32, c1: i32, r2: i32, c2: i32): i32 {
    let dr = Math.abs(r1 - r2);
    let dc = Math.abs(c1 - c2);

    if (dr > rows / 2) dr = rows - dr;
    if (dc > cols / 2) dc = cols - dc;

    return dr * dr + dc * dc;
  }
}
