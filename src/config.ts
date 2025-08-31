import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

export interface LLMSentinelConfig {
  // Server settings
  server: {
    port: number;
    openaiTarget: string;
    ollamaTarget: string;
  };
  
  // Logging configuration
  logging: {
    showDetectedEntity: boolean;          // Show what was detected in logs
    showMaskedValue: boolean;             // Show the masked replacement
    showOccurrenceCount: boolean;         // Show how many times detected
    logToConsole: boolean;                // Log to console
    logToFile: boolean;                   // Log to file
    logLevel: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';
    truncatePayload: boolean;             // Truncate large payloads in logs
    maxPayloadLogLength: number;          // Max length for payload logging
  };
  
  // Sensitive Data Detection settings
  detection: {
    enabled: boolean;                     // Enable/disable sensitive data detection
    enabledRules: string[];               // Which rules to enable
    customRules: Array<{                  // Custom detection rules
      name: string;
      pattern: string;
      replacement: string;
      enabled: boolean;
    }>;
  };
  
  // Notification settings
  notifications: {
    enabled: boolean;                     // Enable desktop notifications
    showEntityType: boolean;              // Show entity type in notification
    showCount: boolean;                   // Show count in notification
    sound: boolean;                       // Play notification sound
  };
  
  // Security settings
  security: {
    redactApiKeys: boolean;               // Redact API keys from logs
    redactCustomHeaders: string[];        // Additional headers to redact
  };
}

export class ConfigManager {
  private configPath: string;
  private config: LLMSentinelConfig;

  constructor(configPath?: string) {
    this.configPath = configPath || this.getDefaultConfigPath();
    this.config = this.loadConfig();
  }

  private getDefaultConfigPath(): string {
    const homeDir = os.homedir();
    const configDir = path.join(homeDir, '.llm-sentinel');
    if (!fs.existsSync(configDir)) {
      fs.mkdirSync(configDir, { recursive: true });
    }
    return path.join(configDir, 'config.json');
  }

  private getDefaultConfig(): LLMSentinelConfig {
    return {
      server: {
        port: 5050,
        openaiTarget: 'https://api.openai.com',
        ollamaTarget: 'http://localhost:11434'
      },
      logging: {
        showDetectedEntity: false,          // Secure by default
        showMaskedValue: true,
        showOccurrenceCount: true,
        logToConsole: true,
        logToFile: true,
        logLevel: 'INFO',
        truncatePayload: true,
        maxPayloadLogLength: 1000
      },
      detection: {
        enabled: true,
        enabledRules: [
          // Core Sensitive Data
          'userPath', 'email', 'ipAddress', 'creditCard', 'ssn', 'phoneNumber',
          // Generic secrets
          'apiKey', 'secret', 'jwtToken',
          // AI/ML API Keys
          'openaiApiKey', 'openaiOrgId', 'openaiProjectId', 'anthropicApiKey', 'claudeApiKey',
          'googleCloudApiKey', 'googleCloudServiceAccount', 'googleCloudProjectId', 'googleCloudCredentials',
          'azureOpenaiApiKey', 'cohereApiKey', 'huggingfaceToken', 'pineconeApiKey', 'weaviateApiKey', 'chromaApiKey',
          // Cloud & Infrastructure
          'awsAccessKey', 'awsSecretKey', 'awsSessionToken', 'azureSubscriptionId', 'gcpProjectNumber',
          'herokuApiKey', 'cloudflareToken', 'firebaseKey',
          // Development & CI/CD
          'githubToken', 'dockerhubToken', 'npmToken', 'pypiToken',
          // Communication & Services
          'twilioApiKey', 'slackToken', 'discordToken', 'sendgridKey', 'mailgunKey', 'stripeKey',
          // Database & Storage
          'databaseUrl', 'redisUrl', 'mongodbConnectionString', 'elasticsearchUrl',
          // Security & Auth
          'sshPrivateKey', 'pgpPrivateKey', 'kubeconfigToken', 'dockerRegistryAuth', 'webhookUrl'
        ],
        customRules: []
      },
      notifications: {
        enabled: true,
        showEntityType: true,
        showCount: true,
        sound: false
      },
      security: {
        redactApiKeys: true,
        redactCustomHeaders: ['x-api-key', 'x-auth-token', 'x-secret']
      }
    };
  }

  private loadConfig(): LLMSentinelConfig {
    try {
      if (fs.existsSync(this.configPath)) {
        const configData = fs.readFileSync(this.configPath, 'utf8');
        const parsedConfig = JSON.parse(configData);
        // Merge with defaults to ensure all properties exist
        return this.mergeWithDefaults(parsedConfig);
      } else {
        // Create default config file
        const defaultConfig = this.getDefaultConfig();
        this.saveConfig(defaultConfig);
        return defaultConfig;
      }
    } catch (error) {
      console.warn(`Failed to load config from ${this.configPath}, using defaults:`, error);
      return this.getDefaultConfig();
    }
  }

  private mergeWithDefaults(config: any): LLMSentinelConfig {
    const defaultConfig = this.getDefaultConfig();
    return {
      server: { ...defaultConfig.server, ...config.server },
      logging: { ...defaultConfig.logging, ...config.logging },
      detection: { 
        ...defaultConfig.detection, 
        ...config.detection,
        customRules: config.detection?.customRules || []
      },
      notifications: { ...defaultConfig.notifications, ...config.notifications },
      security: { 
        ...defaultConfig.security, 
        ...config.security,
        redactCustomHeaders: config.security?.redactCustomHeaders || defaultConfig.security.redactCustomHeaders
      }
    };
  }

  public getConfig(): LLMSentinelConfig {
    return this.config;
  }

  public updateConfig(updates: Partial<LLMSentinelConfig>): void {
    this.config = this.mergeWithDefaults({ ...this.config, ...updates });
    this.saveConfig(this.config);
  }

  public saveConfig(config: LLMSentinelConfig): void {
    try {
      fs.writeFileSync(this.configPath, JSON.stringify(config, null, 2));
    } catch (error) {
      throw new Error(`Failed to save config to ${this.configPath}: ${error}`);
    }
  }

  public getConfigPath(): string {
    return this.configPath;
  }

  public resetConfig(): void {
    const defaultConfig = this.getDefaultConfig();
    this.config = defaultConfig;
    this.saveConfig(defaultConfig);
  }

  // Helper methods for common config operations
  public isRuleEnabled(ruleName: string): boolean {
    return this.config.detection.enabledRules.includes(ruleName);
  }

  public toggleRule(ruleName: string, enabled: boolean): void {
    if (enabled && !this.config.detection.enabledRules.includes(ruleName)) {
      this.config.detection.enabledRules.push(ruleName);
    } else if (!enabled) {
      this.config.detection.enabledRules = this.config.detection.enabledRules.filter(r => r !== ruleName);
    }
    this.saveConfig(this.config);
  }

  public setLogLevel(level: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR'): void {
    this.config.logging.logLevel = level;
    this.saveConfig(this.config);
  }
}