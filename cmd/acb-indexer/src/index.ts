#!/usr/bin/env node
// AI Code Battle Index Builder
// Fetches data from Worker API and generates static JSON index files

import * as fs from 'fs/promises';
import * as path from 'path';
import { exec } from 'child_process';
import { promisify } from 'util';

import 'dotenv/config';
import { ApiClient } from './api.js';
import { IndexGenerator } from './generator.js';
import { FileWriter } from './writer.js';
import type { EvolutionLiveData } from './types.js';

const execAsync = promisify(exec);

interface Config {
  apiUrl: string;
  apiKey: string;
  outputDir: string;
  deployCommand?: string;
  evolutionDataPath?: string;
}

function getConfig(): Config {
  const apiUrl = process.env.API_URL;
  const apiKey = process.env.API_KEY;
  const outputDir = process.env.OUTPUT_DIR || './data';
  const deployCommand = process.env.DEPLOY_COMMAND;
  const evolutionDataPath = process.env.EVOLUTION_DATA_PATH;

  if (!apiUrl) {
    console.error('ERROR: API_URL environment variable is required');
    process.exit(1);
  }

  if (!apiKey) {
    console.error('ERROR: API_KEY environment variable is required');
    process.exit(1);
  }

  return {
    apiUrl,
    apiKey,
    outputDir,
    deployCommand,
    evolutionDataPath,
  };
}

async function runIndexBuilder(config: Config): Promise<void> {
  console.log('AI Code Battle Index Builder');
  console.log('============================');
  console.log(`API URL: ${config.apiUrl}`);
  console.log(`Output directory: ${config.outputDir}`);
  console.log('');

  // Initialize components
  const apiClient = new ApiClient({
    apiUrl: config.apiUrl,
    apiKey: config.apiKey,
  });

  const fileWriter = new FileWriter(config.outputDir);

  // Step 1: Fetch data from API
  console.log('Fetching data from Worker API...');
  const data = await apiClient.fetchExportData();
  console.log(`  - ${data.bots.length} bots`);
  console.log(`  - ${data.matches.length} matches`);
  console.log(`  - ${data.rating_history.length} rating history entries`);
  console.log('');

  // Step 2: Generate index files
  console.log('Generating index files...');
  const generator = new IndexGenerator(data);
  const indexes = generator.generateAll();

  // Step 3: Write files to disk
  console.log('Writing index files...');
  await fileWriter.writeAll(indexes);

  // Step 4: Deploy (optional)
  if (config.deployCommand) {
    console.log('\nDeploying to Cloudflare Pages...');
    try {
      const { stdout, stderr } = await execAsync(config.deployCommand, {
        cwd: config.outputDir,
      });
      if (stdout) console.log(stdout);
      if (stderr) console.error(stderr);
      console.log('Deploy complete!');
    } catch (error) {
      console.error('Deploy failed:', error);
      process.exit(1);
    }
  }
}

async function main(): Promise<void> {
  const config = getConfig();

  try {
    await runIndexBuilder(config);
  } catch (error) {
    console.error('Index builder failed:', error);
    process.exit(1);
  }
}

// Run if executed directly
main();
