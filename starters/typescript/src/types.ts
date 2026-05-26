export interface Position {
  row: number;
  col: number;
}

export type Direction = "N" | "E" | "S" | "W" | "";

export interface VisibleBot {
  position: Position;
  owner: number;
}

export interface VisibleCore {
  position: Position;
  owner: number;
  active: boolean;
}

export interface You {
  id: number;
  energy: number;
  score: number;
}

export interface VisibleState {
  match_id: string;
  turn: number;
  config: Record<string, any>;
  you: You;
  bots: VisibleBot[];
  energy: Position[];
  cores: VisibleCore[];
  walls: Position[];
  dead: VisibleBot[];
}

export interface Move {
  position: Position;
  direction: Direction;
}

export interface TurnResponse {
  moves: Move[];
}
