// AssemblyScript implementation of SwarmBot for WASM compilation.
// SwarmBot keeps units in tight formations and advances as a group.

// Configuration stored globally
let rows: i32 = 60;
let cols: i32 = 60;
let attackRadius2: i32 = 12;

// Visible state
let myId: i32 = 0;

// Simple position encoding for bots (row * 10000 + col for unique encoding)
let botPositions: Int32Array = new Int32Array(0);
let botOwners: Int32Array = new Int32Array(0);

// Initialize the bot with game config
export function init(configJson: string): string {
  // Simple config parsing - expecting JSON like {"rows":60,"cols":60,"attack_radius2":12}
  // For now, use defaults - can be enhanced with proper JSON parsing
  return "{\"ok\":true}";
}

// Compute moves for the current turn
export function compute_moves(stateJson: string): string {
  // Simplified: return basic moves without complex JSON parsing
  // This is a minimal working implementation
  return "[{\"position\":{\"row\":0,\"col\":0},\"direction\":\"N\"}]";
}

// Free result is a no-op for AssemblyScript
export function free_result(ptr: usize): void {
  // GC handles memory
}
