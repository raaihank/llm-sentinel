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
exports.Logger = exports.LogLevel = void 0;
const fs = __importStar(require("fs"));
const path = __importStar(require("path"));
var LogLevel;
(function (LogLevel) {
    LogLevel["DEBUG"] = "DEBUG";
    LogLevel["INFO"] = "INFO";
    LogLevel["WARN"] = "WARN";
    LogLevel["ERROR"] = "ERROR";
})(LogLevel || (exports.LogLevel = LogLevel = {}));
class Logger {
    logFile;
    logToConsole;
    logToFile;
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
    formatMessage(level, message, data) {
        const logEntry = {
            timestamp: new Date().toISOString(),
            level: level,
            message: message,
            ...data
        };
        return JSON.stringify(logEntry);
    }
    writeLog(level, message, data) {
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
    debug(message, data) {
        this.writeLog(LogLevel.DEBUG, message, data);
    }
    log(message, data) {
        this.writeLog(LogLevel.INFO, message, data);
    }
    info(message, data) {
        this.writeLog(LogLevel.INFO, message, data);
    }
    warn(message, data) {
        this.writeLog(LogLevel.WARN, message, data);
    }
    error(message, error) {
        const errorData = error instanceof Error ? {
            message: error.message,
            stack: error.stack
        } : error;
        this.writeLog(LogLevel.ERROR, message, errorData);
    }
    getLogFile() {
        return this.logFile;
    }
}
exports.Logger = Logger;
//# sourceMappingURL=logger.js.map