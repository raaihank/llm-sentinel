import * as fs from 'fs';
import * as path from 'path';

export enum LogLevel {
  DEBUG = 'DEBUG',
  INFO = 'INFO',
  WARN = 'WARN',
  ERROR = 'ERROR'
}

export class Logger {
  private logFile: string;
  private logToConsole: boolean;
  private logToFile: boolean;

  constructor(logToConsole = true, logToFile = true) {
    this.logToConsole = logToConsole;
    this.logToFile = logToFile;
    
    const logDir = path.join(process.cwd(), 'logs');
    if (this.logToFile && !fs.existsSync(logDir)) {
      fs.mkdirSync(logDir, { recursive: true });
    }
    
    const timestamp = new Date().toISOString().split('T')[0];
    this.logFile = path.join(logDir, `llm-sentinel-${timestamp}.log`);
  }

  private formatMessage(level: LogLevel, message: string, data?: any): string {
    const logEntry = {
      timestamp: new Date().toISOString(),
      level: level,
      message: message,
      ...data
    };
    return JSON.stringify(logEntry);
  }

  private writeLog(level: LogLevel, message: string, data?: any): void {
    const formattedMessage = this.formatMessage(level, message, data);
    
    if (this.logToConsole) {
      switch (level) {
        case LogLevel.ERROR:
          console.error(formattedMessage);
          break;
        case LogLevel.WARN:
          console.warn(formattedMessage);
          break;
        default:
          console.log(formattedMessage);
      }
    }
    
    if (this.logToFile) {
      fs.appendFileSync(this.logFile, formattedMessage + '\n');
    }
  }

  public debug(message: string, data?: any): void {
    this.writeLog(LogLevel.DEBUG, message, data);
  }

  public log(message: string, data?: any): void {
    this.writeLog(LogLevel.INFO, message, data);
  }

  public info(message: string, data?: any): void {
    this.writeLog(LogLevel.INFO, message, data);
  }

  public warn(message: string, data?: any): void {
    this.writeLog(LogLevel.WARN, message, data);
  }

  public error(message: string, error?: any): void {
    const errorData = error instanceof Error ? {
      message: error.message,
      stack: error.stack
    } : error;
    this.writeLog(LogLevel.ERROR, message, errorData);
  }

  public getLogFile(): string {
    return this.logFile;
  }
}