interface StartOptions {
    port: string;
    daemon?: boolean;
    openaiTarget: string;
    ollamaTarget: string;
}
export declare function startServer(options: StartOptions): Promise<void>;
export declare function stopServer(): void;
export declare function statusServer(): void;
export declare function restartServer(): void;
export declare function showConfig(): void;
export declare function setConfigValue(key: string, value: string): void;
export declare function resetConfig(): void;
export declare function editConfig(): void;
export declare function listRules(): void;
export declare function toggleRule(ruleName: string, enabled?: boolean): void;
export declare function enableDebugLogging(): void;
export declare function disableDebugLogging(): void;
export declare function toggleNotifications(): void;
export declare function disableAllDetection(): void;
export declare function enableAllDetection(): void;
export declare function setPort(port: string): void;
export declare function quickStatus(): void;
export declare function viewLogs(options: {
    lines: string;
}): void;
export {};
//# sourceMappingURL=commands.d.ts.map