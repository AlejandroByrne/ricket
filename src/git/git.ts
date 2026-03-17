import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import path from 'node:path';

const exec = promisify(execFile);

/**
 * Git audit trail — every vault file operation gets a structured commit.
 * All paths are relative to vaultRoot.
 */
export class GitAudit {
  constructor(private vaultRoot: string) {}

  /**
   * Check if vault root is a git repo.
   * Runs: git rev-parse --git-dir
   */
  async isGitRepo(): Promise<boolean> {
    try {
      await exec('git', ['rev-parse', '--git-dir'], { cwd: this.vaultRoot });
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Stage files and commit with structured message.
   * Message format: "ricket: {action} {details}"
   * Returns false if nothing to commit.
   */
  async commit(files: string[], message: string): Promise<boolean> {
    try {
      // Stage files
      if (files.length > 0) {
        await exec('git', ['add', ...files], { cwd: this.vaultRoot });
      }

      // Commit with --no-gpg-sign to avoid GPG issues
      await exec('git', ['commit', '-m', message, '--no-gpg-sign'], {
        cwd: this.vaultRoot,
      });

      return true;
    } catch (err) {
      const error = err as any;
      // Exit code 1 means nothing to commit
      if (error.code === 1 || error.status === 1) {
        return false;
      }
      // Other errors silently fail (best-effort audit)
      return false;
    }
  }

  /**
   * Stage a file move operation: git rm source, git add destination.
   * Then commit with message: "ricket: filed {source-basename} → {destination}"
   */
  async commitFileMove(source: string, destination: string): Promise<boolean> {
    try {
      const sourceBasename = path.basename(source);

      // Stage deletion and addition
      await exec('git', ['add', source], { cwd: this.vaultRoot });
      await exec('git', ['add', destination], { cwd: this.vaultRoot });

      // Commit with structured message
      const message = `ricket: filed ${sourceBasename} → ${destination}`;
      await exec('git', ['commit', '-m', message, '--no-gpg-sign'], {
        cwd: this.vaultRoot,
      });

      return true;
    } catch (err) {
      const error = err as any;
      // Exit code 1 means nothing to commit
      if (error.code === 1 || error.status === 1) {
        return false;
      }
      // Other errors silently fail (best-effort audit)
      return false;
    }
  }
}
