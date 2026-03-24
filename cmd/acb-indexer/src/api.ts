// API Client for fetching data from Worker API

import type { ApiClientConfig, ExportData } from './types.js';

export class ApiClient {
  private apiUrl: string;
  private apiKey: string;

  constructor(config: ApiClientConfig) {
    this.apiUrl = config.apiUrl.replace(/\/$/, '');
    this.apiKey = config.apiKey;
  }

  /**
   * Fetch all data needed for index building
   */
  async fetchExportData(): Promise<ExportData> {
    const response = await fetch(`${this.apiUrl}/api/data/export`, {
      headers: {
        'X-API-Key': this.apiKey,
        'Accept': 'application/json',
      },
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`API request failed: ${response.status} - ${text}`);
    }

    const result = await response.json() as { success: boolean; data?: ExportData; error?: string };

    if (!result.success) {
      throw new Error(`API returned error: ${result.error}`);
    }

    if (!result.data) {
      throw new Error('API returned no data');
    }

    return result.data;
  }
}
