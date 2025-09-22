export declare enum LogLevel {
    DEBUG = "DEBUG",
    INFO = "INFO",
    WARN = "WARN",
    ERROR = "ERROR"
}
export declare class Logger {
    private logFile;
    private logToConsole;
    private logToFile;
    constructor(logToConsole?: boolean, logToFile?: boolean);
    private formatMessage;
    private writeLog;
    debug(message: string, data?: any): void;
    log(message: string, data?: any): void;
    info(message: string, data?: any): void;
    warn(message: string, data?: any): void;
    error(message: string, error?: any): void;
    getLogFile(): string;
}
//# sourceMappingURL=logger.d.ts.map