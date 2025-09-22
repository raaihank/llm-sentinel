"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.ConfigManager = void 0;
const fs = __importStar(require("fs"));
const path = __importStar(require("path"));
const os = __importStar(require("os"));
class ConfigManager {
    configPath;
    config;
    constructor(configPath) {
        this.configPath = configPath || this.getDefaultConfigPath();
        this.config = this.loadConfig();
    }
    getDefaultConfigPath() {
        const homeDir = os.homedir();
        const configDir = path.join(homeDir, '.llm-sentinel');
        if (!fs.existsSync(configDir)) {
            fs.mkdirSync(configDir, { recursive: true });
        }
        return path.join(configDir, 'config.json');
    }
    getDefaultConfig() {
        return {
            server: {
                port: 5050,
                openaiTarget: 'https://api.openai.com',
                ollamaTarget: 'http://localhost:11434'
            },
            logging: {
                showDetectedEntity: false, // Secure by default
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
    loadConfig() {
        try {
            if (fs.existsSync(this.configPath)) {
                const configData = fs.readFileSync(this.configPath, 'utf8');
                const parsedConfig = JSON.parse(configData);
                // Merge with defaults to ensure all properties exist
                return this.mergeWithDefaults(parsedConfig);
            }
            else {
                // Create default config file
                const defaultConfig = this.getDefaultConfig();
                this.saveConfig(defaultConfig);
                return defaultConfig;
            }
        }
        catch (error) {
            console.warn(`Failed to load config from ${this.configPath}, using defaults:`, error);
            return this.getDefaultConfig();
        }
    }
    mergeWithDefaults(config) {
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
    getConfig() {
        return this.config;
    }
    updateConfig(updates) {
        this.config = this.mergeWithDefaults({ ...this.config, ...updates });
        this.saveConfig(this.config);
    }
    saveConfig(config) {
        try {
            fs.writeFileSync(this.configPath, JSON.stringify(config, null, 2));
        }
        catch (error) {
            throw new Error(`Failed to save config to ${this.configPath}: ${error}`);
        }
    }
    getConfigPath() {
        return this.configPath;
    }
    resetConfig() {
        const defaultConfig = this.getDefaultConfig();
        this.config = defaultConfig;
        this.saveConfig(defaultConfig);
    }
    // Helper methods for common config operations
    isRuleEnabled(ruleName) {
        return this.config.detection.enabledRules.includes(ruleName);
    }
    toggleRule(ruleName, enabled) {
        if (enabled && !this.config.detection.enabledRules.includes(ruleName)) {
            this.config.detection.enabledRules.push(ruleName);
        }
        else if (!enabled) {
            this.config.detection.enabledRules = this.config.detection.enabledRules.filter(r => r !== ruleName);
        }
        this.saveConfig(this.config);
    }
    setLogLevel(level) {
        this.config.logging.logLevel = level;
        this.saveConfig(this.config);
    }
}
exports.ConfigManager = ConfigManager;
//# sourceMappingURL=config.js.map