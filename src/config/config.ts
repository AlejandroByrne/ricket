import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { parse as parseYaml, stringify as stringifyYaml } from 'yaml';

export interface RicketConfig {
  vault: {
    root: string;       // resolved absolute path
    inbox: string;      // relative to root, default "Inbox/"
    archive: string;    // default "Archive/"
    templates: string;  // default "_templates/"
  };
  categories: Category[];
  mcp?: {
    name?: string;      // default "ricket"
    version?: string;   // default from package.json
  };
}

export interface Category {
  name: string;              // e.g. "decision"
  folder: string;            // e.g. "Areas/FCBT Engineering/decisions/"
  template?: string;         // template filename without .md
  naming?: string;           // pattern e.g. "use-{topic}.md"
  tags: string[];            // e.g. ["decision", "fcbt"]
  moc?: string;              // path to MOC file to update
  signals?: string[];        // keywords that hint at this category
}

/**
 * Load ricket.yaml from vault root, resolve all paths, validate schema.
 * Throws descriptive error if file is missing or invalid.
 */
export async function loadConfig(vaultRoot: string): Promise<RicketConfig> {
  const configPath = path.join(vaultRoot, 'ricket.yaml');

  let raw: string;
  try {
    raw = await readFile(configPath, 'utf-8');
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === 'ENOENT') {
      throw new Error(`ricket.yaml not found at ${configPath}`);
    }
    throw new Error(`Failed to read ricket.yaml: ${(err as Error).message}`);
  }

  let parsed: any;
  try {
    parsed = parseYaml(raw);
  } catch (err) {
    throw new Error(`Invalid YAML in ricket.yaml: ${(err as Error).message}`);
  }

  if (!parsed || typeof parsed !== 'object') {
    throw new Error('ricket.yaml must contain a YAML object');
  }

  // Validate vault section
  if (!parsed.vault || typeof parsed.vault !== 'object') {
    throw new Error('ricket.yaml must have a "vault" section');
  }

  const vault = parsed.vault;
  if (!vault.root || typeof vault.root !== 'string') {
    throw new Error('vault.root must be a string');
  }

  // Resolve vault root to absolute path
  const resolvedRoot = path.resolve(vault.root);

  const config: RicketConfig = {
    vault: {
      root: resolvedRoot,
      inbox: vault.inbox || 'Inbox/',
      archive: vault.archive || 'Archive/',
      templates: vault.templates || '_templates/',
    },
    categories: [],
    mcp: parsed.mcp || {},
  };

  // Validate categories
  if (!Array.isArray(parsed.categories)) {
    throw new Error('ricket.yaml must have a "categories" array');
  }

  for (const cat of parsed.categories) {
    if (!cat.name || typeof cat.name !== 'string') {
      throw new Error('Each category must have a "name" string');
    }
    if (!cat.folder || typeof cat.folder !== 'string') {
      throw new Error(`Category "${cat.name}" must have a "folder" string`);
    }
    if (!Array.isArray(cat.tags)) {
      throw new Error(`Category "${cat.name}" must have a "tags" array`);
    }

    config.categories.push({
      name: cat.name,
      folder: cat.folder,
      template: cat.template,
      naming: cat.naming,
      tags: cat.tags,
      moc: cat.moc,
      signals: cat.signals,
    });
  }

  return config;
}

/**
 * Generate a default PARA-based config by scanning existing folders in vaultRoot.
 */
export function generateDefaultConfig(vaultRoot: string): RicketConfig {
  const resolvedRoot = path.resolve(vaultRoot);

  return {
    vault: {
      root: resolvedRoot,
      inbox: 'Inbox/',
      archive: 'Archive/',
      templates: '_templates/',
    },
    categories: [
      {
        name: 'decision',
        folder: 'Areas/FCBT Engineering/decisions/',
        template: 'decision',
        naming: 'use-{topic}.md',
        tags: ['decision', 'fcbt'],
        moc: 'Areas/FCBT Engineering/decisions/MOC.md',
        signals: ['decision', 'standard', 'convention', 'rule'],
      },
      {
        name: 'concept',
        folder: 'Areas/FCBT Engineering/concepts/',
        template: 'concept',
        naming: '{topic}.md',
        tags: ['concept', 'fcbt'],
        moc: 'Areas/FCBT Engineering/concepts/MOC.md',
        signals: ['concept', 'explain', 'understand', 'definition'],
      },
      {
        name: 'project',
        folder: 'Projects/FCBT/',
        template: 'project',
        naming: '{topic}.md',
        tags: ['project', 'fcbt'],
        moc: 'Projects/FCBT/MOC.md',
        signals: ['project', 'task', 'feature', 'initiative'],
      },
      {
        name: 'meeting',
        folder: 'Areas/FCBT Engineering/meetings/',
        template: 'meeting',
        naming: 'YYYY-MM-DD-{topic}.md',
        tags: ['meeting', 'fcbt'],
        signals: ['meeting', 'standup', 'sync', 'planning'],
      },
      {
        name: 'learning',
        folder: 'Areas/Personal Development/',
        template: 'learning',
        naming: '{topic}.md',
        tags: ['learning', 'personal'],
        signals: ['learning', 'skill', 'training', 'course'],
      },
    ],
    mcp: {
      name: 'ricket',
      version: '0.1.0',
    },
  };
}

/**
 * Write config to ricket.yaml at vaultRoot.
 */
export async function writeConfig(config: RicketConfig, vaultRoot: string): Promise<void> {
  const configPath = path.join(vaultRoot, 'ricket.yaml');

  const toWrite: any = {
    vault: {
      root: config.vault.root,
      inbox: config.vault.inbox,
      archive: config.vault.archive,
      templates: config.vault.templates,
    },
    categories: config.categories,
  };

  if (config.mcp && (config.mcp.name || config.mcp.version)) {
    toWrite.mcp = config.mcp;
  }

  const yaml = stringifyYaml(toWrite, { indent: 2 });

  try {
    await writeFile(configPath, yaml, 'utf-8');
  } catch (err) {
    throw new Error(`Failed to write ricket.yaml: ${(err as Error).message}`);
  }
}
