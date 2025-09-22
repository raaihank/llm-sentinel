export interface ProxyServerConfig {
    port: number;
    openaiTarget: string;
    ollamaTarget: string;
}
export declare class ProxyServer {
    private app;
    private server?;
    private wss?;
    private config;
    private appConfig;
    private configManager;
    private sensitiveDataDetector;
    private logger;
    private activeConnections;
    private requestMetrics;
    constructor(config: ProxyServerConfig);
    private broadcastToClients;
    private generateRequestId;
    private onProxyReqHandler;
    private onProxyResHandler;
    private notifySensitiveData;
    private setupProxies;
    start(): Promise<void>;
    private setupWebSocket;
    stop(): Promise<void>;
}
//# sourceMappingURL=proxy-server.d.ts.map