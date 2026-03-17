import { readFile, writeFile, unlink, mkdir } from 'node:fs/promises';
import path from 'node:path';
import { glob } from 'fast-glob';
import { RicketConfig, Category } from '../config/config.js';
import { ParsedNote, parseNote, serializeNote, addFrontmatterTags } from './frontmatter.js';
import { loadTemplate, scaffoldNote, mergeContentIntoTemplate } from './template.js';
import { updateMocFile } from './moc.js';

export interface VaultNote {
  path: string;           // relative to vault root
  absolutePath: string;
  parsed: ParsedNote;
  name: string;           // filename without .md
}

export interface FileNoteOptions {
  source: string;          // relative path of source file (in inbox)
  destination: string;     // relative path of destination
  content?: string;        // override content (if LLM rewrote it)
  tags?: string[];         // tags to ensure are present
  links?: string[];        // wikilinks to add to ## Links section
  moc?: string;            // MOC file to update with a link to this note
  template?: string;       // template name to scaffold with
}

export class Vault {
  constructor(private config: RicketConfig) {}

  /**
   * List all notes in the inbox folder.
   */
  async listInbox(): Promise<VaultNote[]> {
    const inboxPath = path.join(this.config.vault.root, this.config.vault.inbox);
    return this.searchNotes({ folder: this.config.vault.inbox });
  }

  /**
   * Read a single note from vault, returning parsed frontmatter and content.
   */
  async readNote(relativePath: string): Promise<VaultNote> {
    const absolutePath = path.join(this.config.vault.root, relativePath);

    let raw: string;
    try {
      raw = await readFile(absolutePath, 'utf-8');
    } catch (err) {
      throw new Error(`Failed to read note ${relativePath}: ${(err as Error).message}`);
    }

    const parsed = parseNote(raw);
    const name = path.basename(relativePath, '.md');

    return {
      path: relativePath,
      absolutePath,
      parsed,
      name,
    };
  }

  /**
   * Search notes by folder, tags, or content query.
   * Returns all matching notes.
   */
  async searchNotes(options: {
    folder?: string;
    tags?: string[];
    query?: string;
  }): Promise<VaultNote[]> {
    const pattern = options.folder
      ? path.join(this.config.vault.root, options.folder, '**/*.md')
      : path.join(this.config.vault.root, '**/*.md');

    const files = await glob(pattern, {
      absolute: false,
      cwd: this.config.vault.root,
    });

    const results: VaultNote[] = [];

    for (const file of files) {
      try {
        const note = await this.readNote(file);

        // Filter by tags if specified
        if (options.tags && options.tags.length > 0) {
          const noteTags = note.parsed.frontmatter.tags || [];
          const noteTagsArray = Array.isArray(noteTags) ? noteTags : [noteTags];
          const hasAllTags = options.tags.every((tag) => noteTagsArray.includes(tag));
          if (!hasAllTags) {
            continue;
          }
        }

        // Filter by content query if specified
        if (options.query) {
          const content = `${note.parsed.frontmatter.title || ''} ${note.parsed.content}`;
          if (!content.toLowerCase().includes(options.query.toLowerCase())) {
            continue;
          }
        }

        results.push(note);
      } catch {
        // Skip notes that fail to parse
        continue;
      }
    }

    return results;
  }

  /**
   * Get all configured categories.
   */
  async getCategories(): Promise<Category[]> {
    return this.config.categories;
  }

  /**
   * List available templates.
   */
  async getTemplateList(): Promise<string[]> {
    const templatesPath = path.join(this.config.vault.root, this.config.vault.templates);

    try {
      const files = await glob(path.join(templatesPath, '*.md'), {
        absolute: false,
        cwd: this.config.vault.root,
      });

      return files.map((f) => path.basename(f, '.md'));
    } catch {
      return [];
    }
  }

  /**
   * File a note from source to destination.
   * - Reads source
   * - Optionally scaffolds with template
   * - Adds tags to frontmatter
   * - Appends wikilinks to ## Links section
   * - Updates MOC
   * - Writes to destination
   * - Deletes source
   * Returns destination path and git commit message.
   */
  async fileNote(options: FileNoteOptions): Promise<{
    destination: string;
    gitCommitMessage: string;
  }> {
    const sourceAbsPath = path.join(this.config.vault.root, options.source);
    const destAbsPath = path.join(this.config.vault.root, options.destination);

    // Read source
    let sourceNote = await this.readNote(options.source);

    // Use override content if provided
    let content = options.content || sourceNote.parsed.content;

    // Scaffold with template if specified
    if (options.template) {
      const templatesPath = path.join(this.config.vault.root, this.config.vault.templates);
      const templateContent = await loadTemplate(templatesPath, options.template);

      const vars = {
        title: sourceNote.name,
        date: new Date().toISOString().split('T')[0],
      };

      const scaffolded = scaffoldNote(templateContent, vars);
      content = mergeContentIntoTemplate(scaffolded, content);
    }

    // Parse the potentially updated content
    let updated = parseNote(serializeNote(sourceNote.parsed.frontmatter, content));

    // Add tags
    if (options.tags && options.tags.length > 0) {
      updated = addFrontmatterTags(updated, options.tags);
    }

    // Add links to ## Links section
    if (options.links && options.links.length > 0) {
      content = updated.content;

      // Find or create ## Links section
      let linksIndex = content.indexOf('## Links');
      if (linksIndex === -1) {
        // Create new section
        content = content + '\n\n## Links\n' + options.links.map((link) => `- [[${link}]]`).join('\n');
      } else {
        // Find end of Links section (next ## or end of content)
        const afterLinks = content.indexOf('\n\n##', linksIndex + 1);
        const insertPos = afterLinks === -1 ? content.length : afterLinks;

        const linkLines = options.links.map((link: string) => `- [[${link}]]`).join('\n');
        content = content.slice(0, insertPos) + '\n' + linkLines + content.slice(insertPos);
      }

      updated = parseNote(serializeNote(updated.frontmatter, content));
    }

    // Ensure destination directory exists
    const destDir = path.dirname(destAbsPath);
    try {
      await mkdir(destDir, { recursive: true });
    } catch (err) {
      throw new Error(`Failed to create destination directory ${destDir}: ${(err as Error).message}`);
    }

    // Write destination
    const serialized = serializeNote(updated.frontmatter, updated.content);
    try {
      await writeFile(destAbsPath, serialized, 'utf-8');
    } catch (err) {
      throw new Error(`Failed to write destination ${options.destination}: ${(err as Error).message}`);
    }

    // Update MOC if specified
    if (options.moc) {
      const mocAbsPath = path.join(this.config.vault.root, options.moc);
      const noteTitle = sourceNote.name;
      try {
        await updateMocFile(mocAbsPath, noteTitle, options.destination);
      } catch (err) {
        // Log but don't fail the entire operation
        console.warn(`Warning: failed to update MOC: ${(err as Error).message}`);
      }
    }

    // Delete source
    try {
      await unlink(sourceAbsPath);
    } catch (err) {
      throw new Error(`Failed to delete source ${options.source}: ${(err as Error).message}`);
    }

    // Generate git commit message
    const sourceBasename = path.basename(options.source);
    const destBasename = path.basename(options.destination);
    const gitCommitMessage = `ricket: filed ${sourceBasename} → ${options.destination}`;

    return {
      destination: options.destination,
      gitCommitMessage,
    };
  }

  /**
   * Create a new note at destination with content and optional tags/links/moc.
   */
  async createNote(
    destination: string,
    content: string,
    options?: { tags?: string[]; links?: string[]; moc?: string }
  ): Promise<void> {
    const destAbsPath = path.join(this.config.vault.root, destination);

    // Parse content
    let note = parseNote(content);

    // Add tags
    if (options?.tags && options.tags.length > 0) {
      note = addFrontmatterTags(note, options.tags);
    }

    // Add links
    if (options?.links && options.links.length > 0) {
      let noteContent = note.content;

      const linksIndex = noteContent.indexOf('## Links');
      if (linksIndex === -1) {
        noteContent = noteContent + '\n\n## Links\n' + options.links.map((link) => `- [[${link}]]`).join('\n');
      } else {
        const afterLinks = noteContent.indexOf('\n\n##', linksIndex + 1);
        const insertPos = afterLinks === -1 ? noteContent.length : afterLinks;

        const linkLines = options.links.map((link) => `- [[${link}]]`).join('\n');
        noteContent = noteContent.slice(0, insertPos) + '\n' + linkLines + noteContent.slice(insertPos);
      }

      note = parseNote(serializeNote(note.frontmatter, noteContent));
    }

    // Ensure destination directory exists
    const destDir = path.dirname(destAbsPath);
    try {
      await mkdir(destDir, { recursive: true });
    } catch (err) {
      throw new Error(`Failed to create destination directory ${destDir}: ${(err as Error).message}`);
    }

    // Write file
    const serialized = serializeNote(note.frontmatter, note.content);
    try {
      await writeFile(destAbsPath, serialized, 'utf-8');
    } catch (err) {
      throw new Error(`Failed to write note ${destination}: ${(err as Error).message}`);
    }

    // Update MOC if specified
    if (options?.moc) {
      const mocAbsPath = path.join(this.config.vault.root, options.moc);
      const noteTitle = path.basename(destination, '.md');
      try {
        await updateMocFile(mocAbsPath, noteTitle, destination);
      } catch (err) {
        console.warn(`Warning: failed to update MOC: ${(err as Error).message}`);
      }
    }
  }

  /**
   * Update a MOC file by appending a link to a note.
   */
  async updateMoc(mocPath: string, noteTitle: string, notePath: string): Promise<void> {
    const mocAbsPath = path.join(this.config.vault.root, mocPath);
    try {
      await updateMocFile(mocAbsPath, noteTitle, notePath);
    } catch (err) {
      throw new Error(`Failed to update MOC ${mocPath}: ${(err as Error).message}`);
    }
  }

  /**
   * Get all decision notes (categories where name contains "decision").
   */
  async getDecisions(filter?: string): Promise<VaultNote[]> {
    const decisionCategories = this.config.categories.filter((cat) =>
      cat.name.toLowerCase().includes('decision')
    );

    const notes: VaultNote[] = [];

    for (const category of decisionCategories) {
      const categoryNotes = await this.searchNotes({
        folder: category.folder,
        query: filter,
      });

      notes.push(...categoryNotes);
    }

    return notes;
  }

  /**
   * Get vault status: inbox count, total notes, category count.
   */
  async status(): Promise<{
    inboxCount: number;
    totalNotes: number;
    categories: number;
  }> {
    const inboxNotes = await this.listInbox();
    const allNotes = await this.searchNotes({});

    return {
      inboxCount: inboxNotes.length,
      totalNotes: allNotes.length,
      categories: this.config.categories.length,
    };
  }
}
