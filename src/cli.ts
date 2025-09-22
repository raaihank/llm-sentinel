#!/usr/bin/env node

// Suppress specific deprecation warnings
process.removeAllListeners('warning');
process.on('warning', (warning) => {
  // Suppress the specific util._extend deprecation warning from http-proxy-middleware
  if (warning.name === 'DeprecationWarning' && 
      (warning.message.includes('util._extend') || 
       warning.message.includes('DEP0060'))) {
    return;
  }
  // Log other warnings normally
  console.warn(warning.name, warning.message);
});

import { Command } from 'commander';
import { 
  startServer, stopServer, statusServer, restartServer, 
  showConfig, setConfigValue, resetConfig, editConfig, 
  listRules, toggleRule, enableDebugLogging, disableDebugLogging,
  toggleNotifications, disableAllDetection, enableAllDetection,
  setPort, quickStatus
} from './commands';
import { version } from '../package.json';

const program = new Command();

program
  .name('llmsentinel')
  .description('LLM Sentinel - A proxy to detect and mask sensitive information in LLM API requests')
  .version(version);

program
  .command('start')
  .description('Start the LLM Sentinel proxy server')
  .option('-p, --port <port>', 'Port to run the proxy server on', '5050')
  .option('-d, --daemon', 'Run in daemon mode using PM2')
  .option('--openai-target <url>', 'OpenAI API target URL', 'https://api.openai.com')
  .option('--ollama-target <url>', 'Ollama API target URL', 'http://localhost:11434')
  .action(startServer);

program
  .command('stop')
  .description('Stop the LLM Sentinel proxy server')
  .action(stopServer);

program
  .command('status')
  .description('Check the status of LLM Sentinel proxy server')
  .action(statusServer);

program
  .command('restart')
  .description('Restart the LLM Sentinel proxy server')
  .action(restartServer);

program
  .command('logs')
  .description('View logs of the LLM Sentinel proxy server')
  .option('-n, --lines <lines>', 'Number of lines to display', '50')
  .action((options) => {
    const { viewLogs } = require('./commands');
    viewLogs(options);
  });

// Configuration Management Commands
program
  .command('config')
  .description('Show current configuration')
  .action(showConfig);

program
  .command('config:set <key> <value>')
  .description('Set a configuration value (e.g., logging.showDetectedEntity true)')
  .action(setConfigValue);

program
  .command('config:reset')
  .description('Reset configuration to defaults')
  .action(resetConfig);

program
  .command('config:edit')
  .description('Open configuration file in editor')
  .action(editConfig);

// Rule Management Commands  
program
  .command('rules')
  .description('List all PII detection rules and their status')
  .action(listRules);

program
  .command('rules:enable <rule>')
  .description('Enable a PII detection rule')
  .action((rule) => toggleRule(rule, true));

program
  .command('rules:disable <rule>')
  .description('Disable a PII detection rule')
  .action((rule) => toggleRule(rule, false));

program
  .command('rules:toggle <rule>')
  .description('Toggle a PII detection rule on/off')
  .action((rule) => toggleRule(rule));

// Simple Configuration Commands (Most Common)
program
  .command('debug')
  .description('Enable debug logging to see what PII is detected')
  .action(enableDebugLogging);

program
  .command('no-debug')
  .description('Disable debug logging (secure mode)')
  .action(disableDebugLogging);

program
  .command('notifications')
  .description('Toggle desktop notifications on/off')
  .action(toggleNotifications);

program
  .command('protect')
  .description('Enable PII protection (default)')
  .action(enableAllDetection);

program
  .command('no-protect')
  .description('⚠️  Disable ALL PII protection (unsafe)')
  .action(disableAllDetection);

program
  .command('port <port>')
  .description('Set server port (1000-65535)')
  .action(setPort);

program
  .command('info')
  .description('Show quick status and settings')
  .action(quickStatus);

program.parse(process.argv);