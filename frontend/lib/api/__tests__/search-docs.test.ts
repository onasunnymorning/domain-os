import { readdirSync } from 'fs';
import path from 'path';
import { describe, expect, it } from 'vitest';
import { STATIC_DOCS } from '../search';

// Command K finds a docs page only if it has a STATIC_DOCS entry. Workflow
// docs come from the API and are served by app/docs/[key].
describe('command K documentation search', () => {
  it('has an entry for every page under app/docs', () => {
    const docsDir = path.resolve(__dirname, '../../../app/docs');
    const pages = readdirSync(docsDir, { withFileTypes: true })
      .filter((e) => e.isDirectory() && !e.name.startsWith('['))
      .map((e) => e.name);
    const keys = STATIC_DOCS.map((d) => d.key);
    expect(pages.filter((p) => !keys.includes(p))).toEqual([]);
  });
});
