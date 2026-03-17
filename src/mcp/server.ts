import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  type Tool,
} from '@modelcontextprotocol/sdk/types.js';
import { loadConfig, type RicketConfig, type Category } from '../config/config.js';
import { Vault } from '../vault/vault.js';

/**
 * RicketMcpServer exposes ricket vault operations as an MCP server.
 *
 * Capabilities:
 * - List inbox items for triage
 * - Read, search, and create notes
 * - File notes into categories with templates and tags
 * - Query categories and templates
 * - Update MOCs (Indexes of Contents)
 * - Monitor vault status
 */
export class RicketMcpServer {
  private server: Server;
  private vault: Vault | null = null;
  private config: RicketConfig | null = null;

  constructor(private vaultRoot: string) {
    this.server = new Server(
      {
        name: 'ricket',
        version: '0.1.0',
      },
      {
        capabilities: {
          tools: {},
        },
      }
    );
  }

  /**
   * Start the MCP server: load config, initialize vault, register tools, connect stdio transport.
   */
  async start(): Promise<void> {
    try {
      // Load config and initialize vault
      this.config = await loadConfig(this.vaultRoot);
      this.vault = new Vault(this.config);

      // Register tool list handler
      this.server.setRequestHandler(ListToolsRequestSchema, async () => {
        return { tools: this.getToolDefinitions() };
      });

      // Register tool call handler
      this.server.setRequestHandler(CallToolRequestSchema, async (request) => {
        const params = request.params as {
          name: string;
          arguments?: Record<string, unknown>;
        };
        return await this.handleToolCall({
          params: {
            name: params.name,
            arguments: params.arguments || {},
          },
        });
      });

      // Start stdio transport
      const transport = new StdioServerTransport();
      await this.server.connect(transport);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      console.error(`Failed to start ricket MCP server: ${message}`);
      process.exit(1);
    }
  }

  /**
   * Get all tool definitions for the ListTools response.
   */
  private getToolDefinitions(): Tool[] {
    return [
      {
        name: 'vault_list_inbox',
        description:
          'List all files in the vault inbox folder, useful for finding notes to triage into categories. Returns file path, name, and a 200-character preview of the content.',
        inputSchema: {
          type: 'object' as const,
          properties: {},
          required: [],
        },
      },
      {
        name: 'vault_read_note',
        description:
          'Read a complete note from the vault by path. Returns frontmatter (YAML metadata), full content, extracted tags, and wikilinks. Use this to inspect a note before filing or to get full context.',
        inputSchema: {
          type: 'object' as const,
          properties: {
            path: {
              type: 'string',
              description: 'Relative path to the note from vault root (e.g. "Inbox/my-note.md")',
            },
          },
          required: ['path'],
        },
      },
      {
        name: 'vault_search',
        description:
          'Search notes by folder, tags, or text query. Useful for finding existing notes before creating new ones or to understand context around a topic.',
        inputSchema: {
          type: 'object' as const,
          properties: {
            folder: {
              type: 'string',
              description: 'Optional: folder to search in (e.g. "Areas/FCBT Engineering/decisions/")',
            },
            tags: {
              type: 'array',
              items: { type: 'string' },
              description: 'Optional: tags to filter by (matches notes containing ALL specified tags)',
            },
            query: {
              type: 'string',
              description: 'Optional: text search query to match against note content and frontmatter',
            },
          },
          required: [],
        },
      },
      {
        name: 'vault_get_categories',
        description:
          'Get the complete category configuration for this vault. Returns folder paths, naming patterns, templates, MOC paths, and signal keywords for each category.',
        inputSchema: {
          type: 'object' as const,
          properties: {},
          required: [],
        },
      },
      {
        name: 'vault_get_templates',
        description:
          'Get list of available templates with their section headings (fields). Use this to understand what templates exist and what structure they provide.',
        inputSchema: {
          type: 'object' as const,
          properties: {},
          required: [],
        },
      },
      {
        name: 'vault_file_note',
        description:
          'File (move) a note from inbox to a destination category, optionally applying a template, tags, and updating a MOC. This is the core triage operation. Returns the final destination path and git commit message.',
        inputSchema: {
          type: 'object' as const,
          properties: {
            source: {
              type: 'string',
              description: 'Source path relative to vault root (typically in Inbox/)',
            },
            destination: {
              type: 'string',
              description: 'Destination path relative to vault root (e.g. "Areas/FCBT Engineering/decisions/use-dapper-not-efcore.md")',
            },
            content: {
              type: 'string',
              description: 'Optional: new content to write. If omitted, source content is used.',
            },
            tags: {
              type: 'array',
              items: { type: 'string' },
              description: 'Optional: tags to add to frontmatter (e.g. ["decision", "fcbt"])',
            },
            links: {
              type: 'array',
              items: { type: 'string' },
              description: 'Optional: wikilinks to add to frontmatter (e.g. ["decision-template", "decisions-moc"])',
            },
            moc: {
              type: 'string',
              description: 'Optional: MOC path to update after filing (e.g. "Areas/FCBT Engineering/decisions/MOC.md")',
            },
            template: {
              type: 'string',
              description: 'Optional: template name to apply (without .md extension)',
            },
          },
          required: ['source', 'destination'],
        },
      },
      {
        name: 'vault_create_note',
        description:
          'Create a new note at the specified path with the given content. Useful for creating notes directly in their final category. Include tags and links as needed.',
        inputSchema: {
          type: 'object' as const,
          properties: {
            path: {
              type: 'string',
              description: 'Destination path relative to vault root (e.g. "Areas/FCBT Engineering/concepts/sql-server-indexing.md")',
            },
            content: {
              type: 'string',
              description: 'Note content (markdown)',
            },
            tags: {
              type: 'array',
              items: { type: 'string' },
              description: 'Optional: tags to add to frontmatter',
            },
            links: {
              type: 'array',
              items: { type: 'string' },
              description: 'Optional: wikilinks to add to frontmatter',
            },
            moc: {
              type: 'string',
              description: 'Optional: MOC path to update after creation',
            },
          },
          required: ['path', 'content'],
        },
      },
      {
        name: 'vault_status',
        description:
          'Get vault status: number of items in inbox, total note count, and category count. Use this to monitor vault health.',
        inputSchema: {
          type: 'object' as const,
          properties: {},
          required: [],
        },
      },
    ];
  }

  /**
   * Handle incoming tool call requests.
   */
  private async handleToolCall(request: {
    params: {
      name: string;
      arguments: Record<string, unknown>;
    };
  }): Promise<{ content: Array<{ type: 'text'; text: string }> }> {
    const { name, arguments: args } = request.params;

    try {
      if (!this.vault) {
        throw new Error('Vault not initialized');
      }

      let result: unknown;

      switch (name) {
        case 'vault_list_inbox':
          result = await this.handleListInbox();
          break;

        case 'vault_read_note':
          result = await this.handleReadNote(args);
          break;

        case 'vault_search':
          result = await this.handleSearch(args);
          break;

        case 'vault_get_categories':
          result = await this.handleGetCategories();
          break;

        case 'vault_get_templates':
          result = await this.handleGetTemplates();
          break;

        case 'vault_file_note':
          result = await this.handleFileNote(args);
          break;

        case 'vault_create_note':
          result = await this.handleCreateNote(args);
          break;

        case 'vault_status':
          result = await this.handleStatus();
          break;

        default:
          throw new Error(`Unknown tool: ${name}`);
      }

      return {
        content: [
          {
            type: 'text',
            text: JSON.stringify(result, null, 2),
          },
        ],
      };
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      return {
        content: [
          {
            type: 'text',
            text: JSON.stringify(
              {
                error: message,
                tool: name,
              },
              null,
              2
            ),
          },
        ],
      };
    }
  }

  /**
   * vault_list_inbox: List inbox items with preview.
   */
  private async handleListInbox(): Promise<
    Array<{ path: string; name: string; preview: string }>
  > {
    if (!this.vault) throw new Error('Vault not initialized');

    const notes = await this.vault.listInbox();
    return notes.map((note) => ({
      path: note.path,
      name: note.name,
      preview: note.parsed.content.substring(0, 200),
    }));
  }

  /**
   * vault_read_note: Read a complete note with all metadata.
   */
  private async handleReadNote(args: Record<string, unknown>): Promise<{
    path: string;
    name: string;
    frontmatter: Record<string, unknown>;
    content: string;
    tags: string[];
    links: string[];
  }> {
    if (!this.vault) throw new Error('Vault not initialized');

    const path = args.path as string;
    if (!path) throw new Error('Missing required argument: path');

    const note = await this.vault.readNote(path);

    // Extract wikilinks from content: [[link]] or [[link|display]]
    const wikiLinkRegex = /\[\[([^\]|]+)(?:\|[^\]]+)?\]\]/g;
    const links: string[] = [];
    let match;
    while ((match = wikiLinkRegex.exec(note.parsed.content)) !== null) {
      links.push(match[1]);
    }

    return {
      path: note.path,
      name: note.name,
      frontmatter: note.parsed.frontmatter || {},
      content: note.parsed.content,
      tags: (note.parsed.frontmatter?.tags as string[]) || [],
      links,
    };
  }

  /**
   * vault_search: Search notes by folder, tags, or query.
   */
  private async handleSearch(args: Record<string, unknown>): Promise<
    Array<{
      path: string;
      name: string;
      tags: string[];
      preview: string;
    }>
  > {
    if (!this.vault) throw new Error('Vault not initialized');

    const folder = args.folder as string | undefined;
    const tags = (args.tags as string[] | undefined) || undefined;
    const query = args.query as string | undefined;

    const notes = await this.vault.searchNotes({
      folder,
      tags,
      query,
    });

    return notes.map((note) => ({
      path: note.path,
      name: note.name,
      tags: (note.parsed.frontmatter?.tags as string[]) || [],
      preview: note.parsed.content.substring(0, 200),
    }));
  }

  /**
   * vault_get_categories: Return the full category configuration.
   */
  private async handleGetCategories(): Promise<Category[]> {
    if (!this.vault || !this.config) throw new Error('Vault not initialized');
    return this.config.categories;
  }

  /**
   * vault_get_templates: Get list of templates with their fields.
   */
  private async handleGetTemplates(): Promise<
    Array<{ name: string; fields: string[] }>
  > {
    if (!this.vault) throw new Error('Vault not initialized');

    const templateList = await this.vault.getTemplateList();

    // For each template, read it and extract ## Section headings as fields
    const templates: Array<{ name: string; fields: string[] }> = [];

    for (const templateName of templateList) {
      try {
        const templatePath = `${this.config?.vault.templates}${templateName}.md`;
        const note = await this.vault.readNote(templatePath);

        // Extract ## Section headings
        const sectionRegex = /^## (.+?)$/gm;
        const fields: string[] = [];
        let match;
        while ((match = sectionRegex.exec(note.parsed.content)) !== null) {
          fields.push(match[1].trim());
        }

        templates.push({
          name: templateName,
          fields,
        });
      } catch (error) {
        // Skip templates that can't be read
      }
    }

    return templates;
  }

  /**
   * vault_file_note: File a note from source to destination, optionally applying template and tags.
   */
  private async handleFileNote(args: Record<string, unknown>): Promise<{
    destination: string;
    gitCommitMessage: string;
  }> {
    if (!this.vault) throw new Error('Vault not initialized');

    const source = args.source as string;
    const destination = args.destination as string;
    const content = args.content as string | undefined;
    const tags = (args.tags as string[]) || undefined;
    const links = (args.links as string[]) || undefined;
    const moc = args.moc as string | undefined;
    const template = args.template as string | undefined;

    if (!source) throw new Error('Missing required argument: source');
    if (!destination) throw new Error('Missing required argument: destination');

    const result = await this.vault.fileNote({
      source,
      destination,
      content,
      tags,
      links,
      moc,
      template,
    });

    return {
      destination: result.destination,
      gitCommitMessage: result.gitCommitMessage,
    };
  }

  /**
   * vault_create_note: Create a new note.
   */
  private async handleCreateNote(args: Record<string, unknown>): Promise<{
    path: string;
  }> {
    if (!this.vault) throw new Error('Vault not initialized');

    const path = args.path as string;
    const content = args.content as string;
    const tags = (args.tags as string[]) || undefined;
    const links = (args.links as string[]) || undefined;
    const moc = args.moc as string | undefined;

    if (!path) throw new Error('Missing required argument: path');
    if (!content) throw new Error('Missing required argument: content');

    await this.vault.createNote(path, content, {
      tags,
      links,
      moc,
    });

    return { path };
  }

  /**
   * vault_status: Get vault status metrics.
   */
  private async handleStatus(): Promise<{
    inboxCount: number;
    totalNotes: number;
    categories: number;
  }> {
    if (!this.vault || !this.config) throw new Error('Vault not initialized');

    const status = await this.vault.status();

    return {
      inboxCount: status.inboxCount,
      totalNotes: status.totalNotes,
      categories: status.categories,
    };
  }
}
