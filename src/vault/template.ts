import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { parseNote, serializeNote } from './frontmatter.js';

export interface TemplateVars {
  title: string;
  date: string;       // YYYY-MM-DD
  [key: string]: string;
}

/**
 * Load a template file from templatesDir.
 * Template name should not include .md extension.
 */
export async function loadTemplate(templatesDir: string, templateName: string): Promise<string> {
  const templatePath = path.join(templatesDir, `${templateName}.md`);

  try {
    return await readFile(templatePath, 'utf-8');
  } catch (err) {
    throw new Error(`Failed to load template "${templateName}": ${(err as Error).message}`);
  }
}

/**
 * Replace template placeholders with variables.
 * Handles:
 * - <% tp.file.title %> → vars.title
 * - <% tp.date.now("YYYY-MM-DD") %> → vars.date
 * - Other <% tp.date.now("...") %> patterns with vars.date
 */
export function scaffoldNote(templateContent: string, vars: TemplateVars): string {
  let result = templateContent;

  // Replace <% tp.file.title %>
  result = result.replace(/<% tp\.file\.title %>/g, vars.title);

  // Replace <% tp.date.now("...") %> with vars.date
  result = result.replace(/<% tp\.date\.now\("[^"]*"\) %>/g, vars.date);

  return result;
}

/**
 * Merge existing note content into template sections.
 * For each ## Section in the template, look for matching content in existingContent.
 * If found, place it in the template. If not, leave placeholder and mark with <!-- TODO -->.
 */
export function mergeContentIntoTemplate(templateContent: string, existingContent: string): string {
  const templateLines = templateContent.split('\n');
  const existingLines = existingContent.split('\n');

  // Extract sections from template
  interface TemplateSection {
    heading: string;
    index: number;
  }
  const sections: TemplateSection[] = [];
  for (let i = 0; i < templateLines.length; i++) {
    if (templateLines[i].startsWith('## ')) {
      sections.push({
        heading: templateLines[i],
        index: i,
      });
    }
  }

  if (sections.length === 0) {
    // No sections in template, just append existing content
    return templateContent + '\n\n' + existingContent;
  }

  // Extract sections from existing content
  interface ExistingSection {
    heading: string;
    content: string;
  }
  const existingSections: Map<string, ExistingSection> = new Map();
  for (let i = 0; i < existingLines.length; i++) {
    if (existingLines[i].startsWith('## ')) {
      const heading = existingLines[i];
      const endIndex = existingLines.findIndex(
        (line, idx) => idx > i && line.startsWith('## ')
      );
      const contentEnd = endIndex === -1 ? existingLines.length : endIndex;
      const content = existingLines.slice(i + 1, contentEnd).join('\n').trim();

      existingSections.set(heading, { heading, content });
    }
  }

  // Build output by iterating through template and replacing sections
  const output: string[] = [];
  let lastSectionEnd = 0;

  for (let i = 0; i < sections.length; i++) {
    const section = sections[i];
    const nextSectionStart = i + 1 < sections.length ? sections[i + 1].index : templateLines.length;

    // Add everything from last section end to this section
    output.push(...templateLines.slice(lastSectionEnd, section.index));

    // Add section heading
    output.push(section.heading);

    // Check if existing content has this section
    const existing = existingSections.get(section.heading);
    if (existing && existing.content) {
      output.push(existing.content);
    } else {
      // Mark with TODO
      output.push('<!-- TODO -->');
    }

    lastSectionEnd = nextSectionStart;
  }

  // Add any remaining template lines
  output.push(...templateLines.slice(lastSectionEnd));

  return output.join('\n').trimEnd();
}
