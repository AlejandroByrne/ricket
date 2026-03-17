import { readFile, writeFile } from 'node:fs/promises';

/**
 * Update a MOC (Map of Content) file by appending a new wikilink to it.
 * Looks for the last line with existing links (pattern: `- [[`), then appends.
 * If no link lines found, appends at end of file.
 * Returns true if updated, false if MOC file doesn't exist.
 */
export async function updateMocFile(
  mocAbsolutePath: string,
  noteTitle: string,
  notePath: string
): Promise<boolean> {
  let content: string;

  try {
    content = await readFile(mocAbsolutePath, 'utf-8');
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === 'ENOENT') {
      return false;
    }
    throw new Error(`Failed to read MOC file ${mocAbsolutePath}: ${(err as Error).message}`);
  }

  const lines = content.split('\n');
  let insertIndex = lines.length;

  // Find last line that contains a wikilink (pattern: - [[)
  for (let i = lines.length - 1; i >= 0; i--) {
    if (lines[i].includes('- [[')) {
      insertIndex = i + 1;
      break;
    }
  }

  // Build the new wikilink line
  const newLink = `- [[${noteTitle}]]`;

  // Insert the new link
  lines.splice(insertIndex, 0, newLink);

  // Write back to file
  const updated = lines.join('\n');

  try {
    await writeFile(mocAbsolutePath, updated, 'utf-8');
  } catch (err) {
    throw new Error(`Failed to write MOC file ${mocAbsolutePath}: ${(err as Error).message}`);
  }

  return true;
}
