const pm2 = require('pm2');
import * as path from 'path';
import { ProxyServer } from './proxy-server';
import * as fs from 'fs';
import { ConfigManager } from './config';

const APP_NAME = 'llm-sentinel';

interface StartOptions {
  port: string;
  daemon?: boolean;
  openaiTarget: string;
  ollamaTarget: string;
}

export async function startServer(options: StartOptions): Promise<void> {
  const port = parseInt(options.port, 10);
  
  if (options.daemon) {
    // Run with PM2
    startWithPM2(options);
  } else {
    // Run in foreground
    const server = new ProxyServer({
      port,
      openaiTarget: options.openaiTarget,
      ollamaTarget: options.ollamaTarget
    });
    
    try {
      await server.start();
      console.log(`✅ LLM-Sentinel started on port ${port}`);
      console.log(`   OpenAI proxy: http://localhost:${port}/openai`);
      console.log(`   Ollama proxy: http://localhost:${port}/ollama`);
      console.log(`   Health check: http://localhost:${port}/health`);
      console.log('\nPress Ctrl+C to stop the server');
      
      // Handle graceful shutdown
      process.on('SIGINT', async () => {
        console.log('\n\nShutting down LLM-Sentinel...');
        await server.stop();
        process.exit(0);
      });
      
      process.on('SIGTERM', async () => {
        await server.stop();
        process.exit(0);
      });
    } catch (error: any) {
      if (error.message && error.message.includes('EADDRINUSE')) {
        console.error('❌ Port 5050 is already in use');
        console.log('💡 Check if another LLM-Sentinel is running: llmsentinel status');
        console.log('💡 Or use a different port: llmsentinel start -p 5051');
        console.log('💡 To kill existing process: lsof -ti:5050 | xargs kill');
      } else {
        console.error('❌ Failed to start server:', error.message || error);
      }
      process.exit(1);
    }
  }
}

function startWithPM2(options: StartOptions): void {
  pm2.connect((err: any) => {
    if (err) {
      console.error('❌ Failed to connect to PM2:', err);
      process.exit(1);
    }

    const scriptPath = path.join(__dirname, 'server-runner.js');
    
    // Create a simple runner script if it doesn't exist
    const runnerScript = `
const { ProxyServer } = require('./proxy-server');

const server = new ProxyServer({
  port: ${options.port},
  openaiTarget: '${options.openaiTarget}',
  ollamaTarget: '${options.ollamaTarget}'
});

server.start().then(() => {
  console.log('LLM-Sentinel daemon started');
}).catch((error) => {
  console.error('Failed to start:', error);
  process.exit(1);
});

process.on('SIGINT', async () => {
  await server.stop();
  process.exit(0);
});

process.on('SIGTERM', async () => {
  await server.stop();
  process.exit(0);
});
`;

    fs.writeFileSync(scriptPath, runnerScript);

    pm2.start({
      script: scriptPath,
      name: APP_NAME,
      exec_mode: 'fork',
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: '1G',
      env: {
        NODE_ENV: 'production'
      }
    }, (err: any) => {
      if (err) {
        console.error('❌ Failed to start LLM-Sentinel with PM2:', err);
        pm2.disconnect();
        process.exit(1);
      }
      
      console.log(`✅ LLM-Sentinel started in daemon mode on port ${options.port}`);
      console.log(`   OpenAI proxy: http://localhost:${options.port}/openai`);
      console.log(`   Ollama proxy: http://localhost:${options.port}/ollama`);
      console.log(`   Health check: http://localhost:${options.port}/health`);
      console.log('\nUse "llmsentinel status" to check the status');
      console.log('Use "llmsentinel logs" to view logs');
      console.log('Use "llmsentinel stop" to stop the daemon');
      
      pm2.disconnect();
      process.exit(0);
    });
  });
}

export function stopServer(): void {
  console.log('🛑 Stopping LLM-Sentinel (force mode)...');
  
  // Step 1: Try PM2 stop first
  pm2.connect((err: any) => {
    if (!err) {
      pm2.describe(APP_NAME, (err: any, processDescription: any) => {
        if (!err && processDescription && processDescription.length > 0) {
          console.log('📦 Stopping PM2 process...');
          pm2.stop(APP_NAME, () => {
            pm2.delete(APP_NAME, () => {
              pm2.disconnect();
              console.log('✅ PM2 process stopped');
              forceKillProcesses();
            });
          });
        } else {
          pm2.disconnect();
          forceKillProcesses();
        }
      });
    } else {
      forceKillProcesses();
    }
  });

  function forceKillProcesses(): void {
    const { spawn } = require('child_process');
    const os = require('os');
    
    console.log('🔍 Force killing any remaining LLM-Sentinel processes...');
    
    if (os.platform() === 'win32') {
      // Windows: Kill processes by name and port
      spawn('taskkill', ['/F', '/IM', 'node.exe', '/FI', 'WINDOWTITLE eq LLM-Sentinel*'], { stdio: 'ignore' });
      const killByPort = spawn('netstat', ['-ano'], { stdio: ['ignore', 'pipe', 'ignore'] });
      
      killByPort.stdout.on('data', (data: Buffer) => {
        const output = data.toString();
        const lines = output.split('\n');
        lines.forEach((line: string) => {
          if (line.includes(':5050') && line.includes('LISTENING')) {
            const pid = line.trim().split(/\s+/).pop();
            if (pid && !isNaN(parseInt(pid))) {
              spawn('taskkill', ['/F', '/PID', pid], { stdio: 'ignore' });
            }
          }
        });
      });
      
      setTimeout(() => {
        console.log('✅ Force stop completed');
        process.exit(0);
      }, 2000);
      
    } else {
      // Unix/Linux/macOS: Kill by process name and port
      const killCommands = [
        // Kill by process name
        'pkill -f "llm-sentinel\\|LLM-Sentinel"',
        // Kill by port 5050
        'lsof -ti:5050 | xargs kill -9 2>/dev/null || true',
        // Kill any node process running on port 5050
        'lsof -ti tcp:5050 | xargs kill -9 2>/dev/null || true'
      ];
      
      let commandsCompleted = 0;
      
      killCommands.forEach(cmd => {
        const process = spawn('sh', ['-c', cmd], { stdio: 'ignore' });
        process.on('close', () => {
          commandsCompleted++;
          if (commandsCompleted === killCommands.length) {
            console.log('✅ Force stop completed');
            process.exit(0);
          }
        });
      });
      
      // Timeout fallback
      setTimeout(() => {
        console.log('✅ Force stop completed (timeout)');
        process.exit(0);
      }, 3000);
    }
  }
}

export function statusServer(): void {
  pm2.connect((err: any) => {
    if (err) {
      console.error('❌ Failed to connect to PM2:', err);
      console.log('💡 Checking if server is running in foreground mode...');
      
      // Check if port 5050 is in use (foreground mode)
      const http = require('http');
      const req = http.request({
        hostname: 'localhost',
        port: 5050,
        path: '/health',
        method: 'GET',
        timeout: 2000
      }, (_res: any) => {
        console.log('✅ LLM-Sentinel is running in foreground mode');
        console.log('   Use Ctrl+C in the server terminal to stop');
        process.exit(0);
      });
      
      req.on('error', () => {
        console.log('❌ LLM-Sentinel is not running');
        process.exit(1);
      });
      
      req.end();
      return;
    }

    pm2.describe(APP_NAME, (err: any, processDescription: any) => {
      if (err || !processDescription || processDescription.length === 0) {
        console.log('❌ LLM-Sentinel daemon is not running');
        
        // Check if running in foreground mode
        const http = require('http');
        const req = http.request({
          hostname: 'localhost',
          port: 5050,
          path: '/health',
          method: 'GET',
          timeout: 2000
        }, (_res: any) => {
          console.log('✅ LLM-Sentinel is running in foreground mode');
          console.log('   Use Ctrl+C in the server terminal to stop');
          pm2.disconnect();
          process.exit(0);
        });
        
        req.on('error', () => {
          console.log('❌ LLM-Sentinel is not running at all');
          pm2.disconnect();
          process.exit(1);
        });
        
        req.end();
      } else {
        const proc = processDescription[0];
        const status = proc.pm2_env?.status || 'unknown';
        const uptime = proc.pm2_env?.pm_uptime ? 
          new Date(proc.pm2_env.pm_uptime).toLocaleString() : 'unknown';
        const restarts = proc.pm2_env?.restart_time || 0;
        const cpu = proc.monit?.cpu || 0;
        const memory = proc.monit?.memory ? 
          `${Math.round(proc.monit.memory / 1024 / 1024)}MB` : 'unknown';
        
        console.log('📊 LLM-Sentinel Daemon Status:');
        console.log(`   Status: ${status === 'online' ? '✅ Online' : '❌ ' + status}`);
        console.log(`   Started: ${uptime}`);
        console.log(`   Restarts: ${restarts}`);
        console.log(`   CPU: ${cpu}%`);
        console.log(`   Memory: ${memory}`);
        
        pm2.disconnect();
        process.exit(0);
      }
    });
  });
}

export function restartServer(): void {
  pm2.connect((err: any) => {
    if (err) {
      console.error('❌ Failed to connect to PM2:', err);
      process.exit(1);
    }

    pm2.restart(APP_NAME, (err: any) => {
      if (err) {
        console.error('❌ Failed to restart LLM-Sentinel:', err);
      } else {
        console.log('✅ LLM-Sentinel restarted');
      }
      
      pm2.disconnect();
      process.exit(0);
    });
  });
}

// Config Management Commands
export function showConfig(): void {
  const configManager = new ConfigManager();
  const config = configManager.getConfig();
  
  console.log('🔧 LLM-Sentinel Configuration:');
  console.log('─'.repeat(50));
  console.log(`📍 Config file: ${configManager.getConfigPath()}`);
  console.log('\n🖥️  Server Settings:');
  console.log(`   Port: ${config.server.port}`);
  console.log(`   OpenAI Target: ${config.server.openaiTarget}`);
  console.log(`   Ollama Target: ${config.server.ollamaTarget}`);
  
  console.log('\n📋 Logging Settings:');
  console.log(`   Show Detected Entity: ${config.logging.showDetectedEntity ? '✅' : '❌'}`);
  console.log(`   Show Masked Value: ${config.logging.showMaskedValue ? '✅' : '❌'}`);
  console.log(`   Show Occurrence Count: ${config.logging.showOccurrenceCount ? '✅' : '❌'}`);
  console.log(`   Log Level: ${config.logging.logLevel}`);
  console.log(`   Log to Console: ${config.logging.logToConsole ? '✅' : '❌'}`);
  console.log(`   Log to File: ${config.logging.logToFile ? '✅' : '❌'}`);
  
  console.log('\n🛡️  Detection Settings:');
  console.log(`   PII Detection: ${config.detection.enabled ? '✅ Enabled' : '❌ Disabled'}`);
  console.log(`   Active Rules: ${config.detection.enabledRules.length}`);
  console.log(`   Custom Rules: ${config.detection.customRules.length}`);
  
  console.log('\n🔔 Notification Settings:');
  console.log(`   Notifications: ${config.notifications.enabled ? '✅ Enabled' : '❌ Disabled'}`);
  console.log(`   Show Entity Type: ${config.notifications.showEntityType ? '✅' : '❌'}`);
  console.log(`   Show Count: ${config.notifications.showCount ? '✅' : '❌'}`);
  console.log(`   Sound: ${config.notifications.sound ? '✅' : '❌'}`);
  
  console.log('\n🔒 Security Settings:');
  console.log(`   Redact API Keys: ${config.security.redactApiKeys ? '✅' : '❌'}`);
  console.log(`   Custom Headers to Redact: ${config.security.redactCustomHeaders.join(', ')}`);
}

export function setConfigValue(key: string, value: string): void {
  const configManager = new ConfigManager();
  const config = configManager.getConfig();
  
  // Parse the key path (e.g., "logging.showDetectedEntity")
  const keyParts = key.split('.');
  if (keyParts.length !== 2) {
    console.error('❌ Invalid config key format. Use: section.property (e.g., logging.showDetectedEntity)');
    process.exit(1);
  }
  
  const [section, property] = keyParts;
  
  // Parse value based on type
  let parsedValue: any = value;
  if (value === 'true') parsedValue = true;
  else if (value === 'false') parsedValue = false;
  else if (!isNaN(Number(value))) parsedValue = Number(value);
  
  // Update the config
  try {
    if (!(section in config)) {
      throw new Error(`Unknown config section: ${section}`);
    }
    
    const configSection = (config as any)[section];
    if (!(property in configSection)) {
      throw new Error(`Unknown config property: ${property} in section ${section}`);
    }
    
    configSection[property] = parsedValue;
    configManager.saveConfig(config);
    
    console.log(`✅ Updated ${key} = ${parsedValue}`);
    console.log(`📁 Config saved to: ${configManager.getConfigPath()}`);
  } catch (error: any) {
    console.error(`❌ Failed to update config: ${error.message}`);
    process.exit(1);
  }
}

export function resetConfig(): void {
  const configManager = new ConfigManager();
  
  console.log('⚠️  This will reset all configuration to default values.');
  console.log('💡 Use Ctrl+C to cancel, or press Enter to continue...');
  
  process.stdin.once('data', () => {
    try {
      configManager.resetConfig();
      console.log('✅ Configuration reset to defaults');
      console.log(`📁 Config file: ${configManager.getConfigPath()}`);
    } catch (error: any) {
      console.error(`❌ Failed to reset config: ${error.message}`);
      process.exit(1);
    }
  });
}

export function editConfig(): void {
  const configManager = new ConfigManager();
  const configPath = configManager.getConfigPath();
  
  console.log(`📝 Opening config file: ${configPath}`);
  
  // Try to open with terminal editors first, then GUI editors
  const editors = ['nano', 'vim', 'vi', 'code'];
  
  for (const editor of editors) {
    try {
      const { spawn } = require('child_process');
      const child = spawn(editor, [configPath], { stdio: 'inherit' });
      
      child.on('close', (code: number) => {
        if (code === 0) {
          console.log('✅ Config file saved');
          console.log('💡 Restart LLM-Sentinel to apply changes: llmsentinel restart');
        }
      });
      
      child.on('error', () => {
        // Try next editor
      });
      
      return;
    } catch {
      continue;
    }
  }
  
  console.log(`💡 Please manually edit: ${configPath}`);
  console.log('💡 Restart LLM-Sentinel to apply changes: llmsentinel restart');
}

export function listRules(): void {
  const configManager = new ConfigManager();
  const config = configManager.getConfig();
  
  console.log('📋 Available Sensitive Data Detection Rules:');
  console.log('─'.repeat(50));
  
  const allRules = [
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
  ];
  
  allRules.forEach(rule => {
    const enabled = config.detection.enabledRules.includes(rule);
    console.log(`   ${enabled ? '✅' : '❌'} ${rule}`);
  });
  
  if (config.detection.customRules.length > 0) {
    console.log('\n🎯 Custom Rules:');
    config.detection.customRules.forEach(rule => {
      console.log(`   ${rule.enabled ? '✅' : '❌'} ${rule.name} (custom)`);
    });
  }
}

export function toggleRule(ruleName: string, enabled?: boolean): void {
  const configManager = new ConfigManager();
  const config = configManager.getConfig();
  
  const allRules = [
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
  ];
  
  if (!allRules.includes(ruleName)) {
    console.error(`❌ Unknown rule: ${ruleName}`);
    console.log('💡 Available rules:', allRules.join(', '));
    process.exit(1);
  }
  
  const isCurrentlyEnabled = config.detection.enabledRules.includes(ruleName);
  const shouldEnable = enabled !== undefined ? enabled : !isCurrentlyEnabled;
  
  configManager.toggleRule(ruleName, shouldEnable);
  
  console.log(`${shouldEnable ? '✅ Enabled' : '❌ Disabled'} rule: ${ruleName}`);
  console.log('💡 Restart LLM-Sentinel to apply changes: llmsentinel restart');
}

// Simple configuration commands for common use cases
export function enableDebugLogging(): void {
  const configManager = new ConfigManager();
  configManager.setLogLevel('DEBUG');
  configManager.updateConfig({
    logging: {
      ...configManager.getConfig().logging,
      showDetectedEntity: true,
      showMaskedValue: true,
      showOccurrenceCount: true
    }
  });
  console.log('✅ Debug logging enabled - you will see what sensitive data is detected');
  console.log('💡 Restart LLM-Sentinel: llmsentinel restart');
}

export function disableDebugLogging(): void {
  const configManager = new ConfigManager();
  configManager.setLogLevel('INFO');
  configManager.updateConfig({
    logging: {
      ...configManager.getConfig().logging,
      showDetectedEntity: false,
      showMaskedValue: true,
      showOccurrenceCount: true
    }
  });
  console.log('✅ Debug logging disabled - sensitive data details hidden for security');
  console.log('💡 Restart LLM-Sentinel: llmsentinel restart');
}

export function toggleNotifications(): void {
  const configManager = new ConfigManager();
  const config = configManager.getConfig();
  const newValue = !config.notifications.enabled;
  
  configManager.updateConfig({
    notifications: {
      ...config.notifications,
      enabled: newValue
    }
  });
  
  console.log(`${newValue ? '🔔 Notifications enabled' : '🔕 Notifications disabled'}`);
  console.log('💡 Restart LLM-Sentinel: llmsentinel restart');
}

export function disableAllDetection(): void {
  console.log('⚠️  This will disable ALL sensitive data detection - your data will NOT be protected!');
  console.log('💡 Use Ctrl+C to cancel, or press Enter to continue...');
  
  process.stdin.once('data', () => {
    const configManager = new ConfigManager();
    configManager.updateConfig({
      detection: {
        ...configManager.getConfig().detection,
        enabled: false
      }
    });
    console.log('❌ Data detection disabled - NO sensitive data will be masked!');
    console.log('💡 Restart LLM-Sentinel: llmsentinel restart');
  });
}

export function enableAllDetection(): void {
  const configManager = new ConfigManager();
  configManager.updateConfig({
    detection: {
      ...configManager.getConfig().detection,
      enabled: true
    }
  });
  console.log('✅ Data detection enabled - your sensitive data will be protected');
  console.log('💡 Restart LLM-Sentinel: llmsentinel restart');
}

export function setPort(port: string): void {
  const portNum = parseInt(port, 10);
  if (isNaN(portNum) || portNum < 1000 || portNum > 65535) {
    console.error('❌ Invalid port number. Use a number between 1000-65535');
    process.exit(1);
  }
  
  const configManager = new ConfigManager();
  configManager.updateConfig({
    server: {
      ...configManager.getConfig().server,
      port: portNum
    }
  });
  
  console.log(`✅ Port set to ${portNum}`);
  console.log('💡 Restart LLM-Sentinel: llmsentinel restart');
}

export function quickStatus(): void {
  const configManager = new ConfigManager();
  const config = configManager.getConfig();
  
  console.log('⚡ LLM-Sentinel Quick Status:');
  console.log('─'.repeat(40));
  console.log(`🛡️  Data Protection: ${config.detection.enabled ? '✅ ON' : '❌ OFF'}`);
  console.log(`🔔 Notifications: ${config.notifications.enabled ? '✅ ON' : '❌ OFF'}`);
  console.log(`📋 Debug Logging: ${config.logging.showDetectedEntity ? '✅ ON' : '❌ OFF'}`);
  console.log(`🚪 Port: ${config.server.port}`);
  console.log(`📝 Active Rules: ${config.detection.enabledRules.length}/41`);
  console.log(`📁 Config: ${configManager.getConfigPath()}`);
}

export function viewLogs(options: { lines: string }): void {
  pm2.connect((err: any) => {
    if (err) {
      // PM2 not available, check for local log files
      console.log('💡 PM2 not available. Checking local log files...');
      
      const today = new Date().toISOString().split('T')[0];
      const logFile = path.join(process.cwd(), 'logs', `llm-sentinel-${today}.log`);
      
      if (fs.existsSync(logFile)) {
        console.log('📝 Recent logs from local file:');
        console.log('─'.repeat(50));
        
        const logs = fs.readFileSync(logFile, 'utf-8');
        const lines = logs.split('\n');
        const numLines = parseInt(options.lines, 10);
        const recentLines = lines.slice(-numLines).filter(line => line.trim());
        console.log(recentLines.join('\n'));
      } else {
        console.log('❌ No log files found');
        console.log('💡 For foreground mode, logs appear in the terminal where you started the server');
        console.log('💡 For daemon mode, use: llmsentinel start -d');
      }
      process.exit(0);
      return;
    }

    pm2.describe(APP_NAME, (err: any, processDescription: any) => {
      if (err || !processDescription || processDescription.length === 0) {
        console.log('❌ LLM-Sentinel daemon is not running');
        
        // Check local log files as fallback
        const today = new Date().toISOString().split('T')[0];
        const logFile = path.join(process.cwd(), 'logs', `llm-sentinel-${today}.log`);
        
        if (fs.existsSync(logFile)) {
          console.log('📝 Recent logs from local file:');
          console.log('─'.repeat(50));
          
          const logs = fs.readFileSync(logFile, 'utf-8');
          const lines = logs.split('\n');
          const numLines = parseInt(options.lines, 10);
          const recentLines = lines.slice(-numLines).filter(line => line.trim());
          console.log(recentLines.join('\n'));
        } else {
          console.log('💡 For foreground mode, logs appear in the terminal where you started the server');
        }
        
        pm2.disconnect();
        process.exit(1);
        return;
      }

      const proc = processDescription[0];
      const logFile = proc.pm2_env?.pm_out_log_path;
      const errorFile = proc.pm2_env?.pm_err_log_path;
      
      if (logFile && fs.existsSync(logFile)) {
        console.log('📝 Recent PM2 logs:');
        console.log('─'.repeat(50));
        
        const logs = fs.readFileSync(logFile, 'utf-8');
        const lines = logs.split('\n');
        const numLines = parseInt(options.lines, 10);
        const recentLines = lines.slice(-numLines).join('\n');
        console.log(recentLines);
      }
      
      if (errorFile && fs.existsSync(errorFile)) {
        const errors = fs.readFileSync(errorFile, 'utf-8');
        if (errors.trim()) {
          console.log('\n❌ Recent errors:');
          console.log('─'.repeat(50));
          const lines = errors.split('\n');
          const numLines = parseInt(options.lines, 10);
          const recentLines = lines.slice(-numLines).join('\n');
          console.log(recentLines);
        }
      }
      
      pm2.disconnect();
      process.exit(0);
    });
  });
}