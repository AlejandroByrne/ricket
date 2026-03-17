import initSqlJs, { Database } from 'sql.js';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import path from 'node:path';

export interface NoteRecord {
  path: string;
  title: string;
  tags: string[];
  content: string;
}

export interface SearchResult {
  path: string;
  title: string;
  snippet?: string;
}

export interface FolderResult {
  path: string;
  title: string;
  tags: string[];
}

/**
 * SQLite index for fast vault search and context retrieval.
 * Uses sql.js (pure JS, no native deps).
 */
export class VaultIndex {
  private db: Database | null = null;
  private indexPath: string;

  constructor(private vaultRoot: string) {
    this.indexPath = path.join(vaultRoot, '.ricket', 'index.db');
  }

  /**
   * Initialize the database (create .ricket/index.db or use existing).
   */
  async init(): Promise<void> {
    const SQL = await initSqlJs();

    // Try to load existing index
    try {
      const data = await readFile(this.indexPath);
      this.db = new SQL.Database(new Uint8Array(data));
    } catch {
      // Create new in-memory database
      this.db = new SQL.Database();
      this.createSchema();
    }
  }

  /**
   * Create the notes table schema.
   */
  private createSchema(): void {
    if (!this.db) {
      throw new Error('Database not initialized');
    }

    this.db.run(`
      CREATE TABLE IF NOT EXISTS notes (
        path TEXT PRIMARY KEY,
        title TEXT NOT NULL,
        tags TEXT,
        content TEXT,
        folder TEXT,
        updated_at TEXT
      );
    `);

    this.db.run(`
      CREATE INDEX IF NOT EXISTS idx_folder ON notes(folder);
    `);
  }

  /**
   * Rebuild index by scanning all .md files in the vault.
   */
  async rebuild(notes: NoteRecord[]): Promise<void> {
    if (!this.db) {
      throw new Error('Database not initialized');
    }

    // Clear existing data
    this.db.run('DELETE FROM notes;');

    // Insert all notes
    for (const note of notes) {
      const folder = this.extractFolder(note.path);
      const tagsJson = JSON.stringify(note.tags);
      const now = new Date().toISOString();

      this.db.run(
        `INSERT INTO notes (path, title, tags, content, folder, updated_at)
         VALUES (?, ?, ?, ?, ?, ?)`,
        [note.path, note.title, tagsJson, note.content, folder, now]
      );
    }

    // Persist to disk
    await this.persist();
  }

  /**
   * Search by tags (array of tag strings).
   * Returns notes that have ALL specified tags.
   */
  async searchByTags(tags: string[]): Promise<SearchResult[]> {
    if (!this.db) {
      throw new Error('Database not initialized');
    }

    if (tags.length === 0) {
      return [];
    }

    const results: SearchResult[] = [];

    // Get all notes
    const stmt = this.db.prepare('SELECT path, title, tags FROM notes');
    while (stmt.step()) {
      const [path, title, tagsJson] = stmt.getAsObject(['path', 'title', 'tags']) as [string, string, string];

      try {
        const noteTags = JSON.parse(tagsJson || '[]') as string[];

        // Check if note has all requested tags
        if (tags.every((tag) => noteTags.includes(tag))) {
          results.push({ path, title });
        }
      } catch {
        // Skip notes with invalid tags JSON
      }
    }

    stmt.free();
    return results;
  }

  /**
   * Full-text search on content.
   * Returns snippet: 100 chars around first match.
   */
  async searchContent(query: string): Promise<SearchResult[]> {
    if (!this.db) {
      throw new Error('Database not initialized');
    }

    if (!query.trim()) {
      return [];
    }

    const results: SearchResult[] = [];
    const searchPattern = `%${query}%`;

    // Query notes where content matches
    const stmt = this.db.prepare('SELECT path, title, content FROM notes WHERE content LIKE ?');
    stmt.bind([searchPattern]);

    while (stmt.step()) {
      const row = stmt.getAsObject(['path', 'title', 'content']) as {
        path: string;
        title: string;
        content: string;
      };

      // Generate snippet: 100 chars around first match
      const lowerContent = row.content.toLowerCase();
      const lowerQuery = query.toLowerCase();
      const matchIndex = lowerContent.indexOf(lowerQuery);

      let snippet = '';
      if (matchIndex !== -1) {
        const start = Math.max(0, matchIndex - 50);
        const end = Math.min(row.content.length, matchIndex + 50);
        snippet = row.content.substring(start, end).trim();
        if (start > 0) {
          snippet = '...' + snippet;
        }
        if (end < row.content.length) {
          snippet = snippet + '...';
        }
      }

      results.push({
        path: row.path,
        title: row.title,
        snippet,
      });
    }

    stmt.free();
    return results;
  }

  /**
   * Get all notes in a folder.
   */
  async getByFolder(folder: string): Promise<FolderResult[]> {
    if (!this.db) {
      throw new Error('Database not initialized');
    }

    const results: FolderResult[] = [];

    // Query notes in folder (allow prefix matching)
    const searchFolder = folder.endsWith('/') ? folder : folder + '/';
    const stmt = this.db.prepare('SELECT path, title, tags FROM notes WHERE folder LIKE ?');
    stmt.bind([searchFolder + '%']);

    while (stmt.step()) {
      const row = stmt.getAsObject(['path', 'title', 'tags']) as {
        path: string;
        title: string;
        tags: string;
      };

      try {
        const tags = JSON.parse(row.tags || '[]') as string[];
        results.push({
          path: row.path,
          title: row.title,
          tags,
        });
      } catch {
        // Skip notes with invalid tags JSON
      }
    }

    stmt.free();
    return results;
  }

  /**
   * Close the database and persist to disk.
   */
  async close(): Promise<void> {
    if (this.db) {
      await this.persist();
      this.db.close();
      this.db = null;
    }
  }

  /**
   * Close without persisting (for testing).
   */
  closeSync(): void {
    if (this.db) {
      this.db.close();
      this.db = null;
    }
  }

  /**
   * Persist database to .ricket/index.db.
   */
  private async persist(): Promise<void> {
    if (!this.db) {
      return;
    }

    try {
      const data = this.db.export();
      const buffer = Buffer.from(data);

      // Ensure .ricket directory exists
      const dir = path.dirname(this.indexPath);
      await mkdir(dir, { recursive: true });

      await writeFile(this.indexPath, buffer);
    } catch (err) {
      // Best-effort persistence
    }
  }

  /**
   * Extract folder from path (e.g., "Projects/FCBT/MOC.md" → "Projects/FCBT/")
   */
  private extractFolder(notePath: string): string {
    const parts = notePath.split('/');
    if (parts.length <= 1) {
      return '';
    }
    return parts.slice(0, -1).join('/') + '/';
  }
}
