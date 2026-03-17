#!/usr/bin/env node
import { Command } from 'commander';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import path from 'node:path';
import { glob } from 'fast-glob';
import { loadConfig, generateDefaultConfig, writeConfig } from '../config/config.js';
import { Vault } from '../vault/vault.js';
import { RicketMcpServer } from '../mcp/server.js';

const pkg = JSON.parse(
  await readFile(new URL('../../package.json', import.meta.url), 'utf-8')
) as { version: string };

const program = new Command();

program
  .name('ricket')
  .description('Vault-powered context engine for AI coding agents')
  .version(pkg.version)
  .option('-r, --vault-root <path>', 'Vault root directory', process.cwd());

/**
 * ricket init [path]
 * Initialize ricket in a vault directory.
 */
program
  .command('init [path]')
  .description('Initialize ricket in a vault directory')
  .action(async (inputPath?: string) => {
    const vaultRoot = inputPath ? path.resolve(inputPath) : process.cwd();

    // Check if ricket.yaml already exists
    const configPath = path.join(vaultRoot, 'ricket.yaml');
    if (existsSync(configPath)) {
      console.error(`Error: ricket.yaml already exists at ${configPath}`);
      process.exit(1);
    }

    try {
      // Scan for existing PARA folders
      const paras = ['Projects', 'Areas', 'Resources', 'Archive'];
      const foundParas = [];

      for (const para of paras) {
        const paraPath = path.join(vaultRoot, para);
        if (existsSync(paraPath)) {
          foundParas.push(para);
        }
      }

      // Check for _templates folder
      const templatesPath = path.join(vaultRoot, '_templates');
      const hasTemplates = existsSync(templatesPath);

      // Generate default config
      const config = generateDefaultConfig(vaultRoot);

      // Create .ricket directory
      const ricketDir = path.join(vaultRoot, '.ricket');
      await mkdir(ricketDir, { recursive: true });

      // Write config
      await writeConfig(config, vaultRoot);

      // Print summary
      const categoryCount = config.categories.length;
      console.log(`Initialized ricket in ${vaultRoot}`);
      console.log(`Found PARA folders: ${foundParas.length > 0 ? foundParas.join(', ') : 'none'}`);
      console.log(`Found _templates: ${hasTemplates ? 'yes' : 'no'}`);
      console.log(`Configured categories: ${categoryCount}`);
      console.log(`Config written to: ricket.yaml`);
    } catch (err) {
      console.error(`Error: Failed to initialize ricket: ${(err as Error).message}`);
      process.exit(1);
    }
  });

/**
 * ricket serve
 * Start MCP server on stdio.
 */
program
  .command('serve')
  .description('Start ricket MCP server on stdio')
  .action(async () => {
    const options = program.opts() as { vaultRoot?: string };
    const vaultRoot = options.vaultRoot || process.cwd();

    try {
      // Verify ricket.yaml exists
      const configPath = path.join(vaultRoot, 'ricket.yaml');
      if (!existsSync(configPath)) {
        console.error(
          `Error: No ricket.yaml found at ${vaultRoot}. Run 'ricket init' first.`
        );
        process.exit(1);
      }

      // Start MCP server
      const server = new RicketMcpServer(vaultRoot);
      console.error(`ricket MCP server running (vault: ${vaultRoot})`);
      await server.start();
    } catch (err) {
      console.error(`Error: Failed to start MCP server: ${(err as Error).message}`);
      process.exit(1);
    }
  });

/**
 * ricket status
 * Print vault statistics.
 */
program
  .command('status')
  .description('Print vault status and statistics')
  .action(async () => {
    const options = program.opts() as { vaultRoot?: string };
    const vaultRoot = options.vaultRoot || process.cwd();

    try {
      // Load config
      const config = await loadConfig(vaultRoot);
      const vault = new Vault(config);

      // Get status
      const status = await vault.status();

      // Print stats
      console.log(`Vault: ${vaultRoot}`);
      console.log(`Categories: ${status.categories}`);
      console.log(`Total notes: ${status.totalNotes}`);
      console.log(`Inbox: ${status.inboxCount} item${status.inboxCount === 1 ? '' : 's'}`);

      // List inbox if not empty
      if (status.inboxCount > 0) {
        console.log('');
        console.log('Inbox:');
        const inboxNotes = await vault.listInbox();
        for (const note of inboxNotes) {
          console.log(`  - ${note.path}`);
        }
      }
    } catch (err) {
      console.error(`Error: ${(err as Error).message}`);
      process.exit(1);
    }
  });

program.parse();
