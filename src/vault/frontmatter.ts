import { parse as parseYaml, stringify as stringifyYaml } from 'yaml';

export interface ParsedNote {
  frontmatter: Record<string, any>;  // parsed YAML
  content: string;                     // everything after frontmatter
  raw: string;                         // original full text
}

/**
 * Parse a markdown note into frontmatter, content, and raw.
 * Frontmatter is YAML between --- delimiters at the start.
 */
export function parseNote(raw: string): ParsedNote {
  const lines = raw.split('\n');

  // Check if first line is ---
  if (lines.length === 0 || lines[0].trim() !== '---') {
    return {
      frontmatter: {},
      content: raw,
      raw,
    };
  }

  // Find closing --- delimiter
  let closeIndex = -1;
  for (let i = 1; i < lines.length; i++) {
    if (lines[i].trim() === '---') {
      closeIndex = i;
      break;
    }
  }

  if (closeIndex === -1) {
    // No closing delimiter, treat entire content as raw
    return {
      frontmatter: {},
      content: raw,
      raw,
    };
  }

  // Extract frontmatter YAML
  const frontmatterLines = lines.slice(1, closeIndex);
  const frontmatterText = frontmatterLines.join('\n');

  let frontmatter: Record<string, any> = {};
  if (frontmatterText.trim()) {
    try {
      const parsed = parseYaml(frontmatterText);
      if (parsed && typeof parsed === 'object') {
        frontmatter = parsed;
      }
    } catch {
      // If YAML parsing fails, treat as empty frontmatter
      frontmatter = {};
    }
  }

  // Extract content (everything after closing ---)
  const content = lines.slice(closeIndex + 1).join('\n').trimStart();

  return {
    frontmatter,
    content,
    raw,
  };
}

/**
 * Serialize frontmatter and content back to markdown.
 */
export function serializeNote(frontmatter: Record<string, any>, content: string): string {
  if (Object.keys(frontmatter).length === 0) {
    return content;
  }

  const yaml = stringifyYaml(frontmatter, { indent: 2 });
  return `---\n${yaml}---\n${content}`;
}

/**
 * Merge tags arrays, removing duplicates, preserving order.
 */
export function mergeTags(existing: string[], toAdd: string[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];

  for (const tag of existing) {
    if (!seen.has(tag)) {
      seen.add(tag);
      result.push(tag);
    }
  }

  for (const tag of toAdd) {
    if (!seen.has(tag)) {
      seen.add(tag);
      result.push(tag);
    }
  }

  return result;
}

/**
 * Add tags to a parsed note's frontmatter, handling both array and list formats.
 */
export function addFrontmatterTags(note: ParsedNote, tags: string[]): ParsedNote {
  const current = note.frontmatter.tags || [];

  // Normalize to array
  let currentArray: string[] = [];
  if (Array.isArray(current)) {
    currentArray = current;
  } else if (typeof current === 'string') {
    currentArray = [current];
  }

  const merged = mergeTags(currentArray, tags);

  return {
    ...note,
    frontmatter: {
      ...note.frontmatter,
      tags: merged,
    },
  };
}
