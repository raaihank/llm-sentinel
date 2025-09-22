export interface LLMSentinelConfig {
    server: {
        port: number;
        openaiTarget: string;
        ollamaTarget: string;
    };
    logging: {
        showDetectedEntity: boolean;
        showMaskedValue: boolean;
        showOccurrenceCount: boolean;
        logToConsole: boolean;
        logToFile: boolean;
        logLevel: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';
        truncatePayload: boolean;
        maxPayloadLogLength: number;
    };
    detection: {
        enabled: boolean;
        enabledRules: string[];
        customRules: Array<{
            name: string;
            pattern: string;
            replacement: string;
            enabled: boolean;
        }>;
    };
    notifications: {
        enabled: boolean;
        showEntityType: boolean;
        showCount: boolean;
        sound: boolean;
    };
    security: {
        redactApiKeys: boolean;
        redactCustomHeaders: string[];
    };
}
export declare class ConfigManager {
    private configPath;
    private config;
    constructor(configPath?: string);
    private getDefaultConfigPath;
    private getDefaultConfig;
    private loadConfig;
    private mergeWithDefaults;
    getConfig(): LLMSentinelConfig;
    updateConfig(updates: Partial<LLMSentinelConfig>): void;
    saveConfig(config: LLMSentinelConfig): void;
    getConfigPath(): string;
    resetConfig(): void;
    isRuleEnabled(ruleName: string): boolean;
    toggleRule(ruleName: string, enabled: boolean): void;
    setLogLevel(level: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR'): void;
}
//# sourceMappingURL=config.d.ts.map