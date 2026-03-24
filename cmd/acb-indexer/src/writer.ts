// File Writer - Writes generated index files to disk

import * as fs from 'fs/promises';
import * as path from 'path';

import type { LeaderboardIndex, BotDirectory, BotProfile, MatchIndex } from './types.js';

export class FileWriter {
  private outputDir: string;

  constructor(outputDir: string) {
    this.outputDir = outputDir;
  }

  /**
   * Ensure output directory structure exists
   */
  async ensureDirectories(): Promise<void> {
    const dirs = [
      this.outputDir,
      path.join(this.outputDir, 'bots'),
      path.join(this.outputDir, 'matches'),
    ];

    for (const dir of dirs) {
      try {
        await fs.mkdir(dir, { recursive: true });
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code !== 'EEXIST') {
          throw error;
        }
      }
    }
  }

  /**
   * Write JSON file
   */
  private async writeJson(filePath: string, data: unknown): Promise<void> {
    const content = JSON.stringify(data, null, 2);
    await fs.writeFile(filePath, content, 'utf-8');
    console.log(`Wrote: ${filePath}`);
  }

  /**
   * Write leaderboard.json
   */
  async writeLeaderboard(leaderboard: LeaderboardIndex): Promise<void> {
    const filePath = path.join(this.outputDir, 'leaderboard.json');
    await this.writeJson(filePath, leaderboard);
  }

  /**
   * Write bots/index.json
   */
  async writeBotDirectory(directory: BotDirectory): Promise<void> {
    const filePath = path.join(this.outputDir, 'bots', 'index.json');
    await this.writeJson(filePath, directory);
  }

  /**
   * Write individual bot profile
   */
  async writeBotProfile(botId: string, profile: BotProfile): Promise<void> {
    const filePath = path.join(this.outputDir, 'bots', `${botId}.json`);
    await this.writeJson(filePath, profile);
  }

  /**
   * Write all bot profiles
   */
  async writeBotProfiles(profiles: Map<string, BotProfile>): Promise<void> {
    const writePromises: Promise<void>[] = [];

    for (const [botId, profile] of profiles) {
      writePromises.push(this.writeBotProfile(botId, profile));
    }

    await Promise.all(writePromises);
  }

  /**
   * Write matches/index.json
   */
  async writeMatchIndex(matchIndex: MatchIndex): Promise<void> {
    const filePath = path.join(this.outputDir, 'matches', 'index.json');
    await this.writeJson(filePath, matchIndex);
  }

  /**
   * Write all index files
   */
  async writeAll(data: {
    leaderboard: LeaderboardIndex;
    botDirectory: BotDirectory;
    botProfiles: Map<string, BotProfile>;
    matchIndex: MatchIndex;
  }): Promise<void> {
    await this.ensureDirectories();

    await this.writeLeaderboard(data.leaderboard);
    await this.writeBotDirectory(data.botDirectory);
    await this.writeBotProfiles(data.botProfiles);
    await this.writeMatchIndex(data.matchIndex);

    console.log(`\nIndex generation complete!`);
    console.log(`  - ${data.leaderboard.entries.length} leaderboard entries`);
    console.log(`  - ${data.botProfiles.size} bot profiles`);
    console.log(`  - ${data.matchIndex.matches.length} matches`);
  }
}
