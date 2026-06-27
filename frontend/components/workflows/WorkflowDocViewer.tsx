'use client';

import { useEffect, useRef, memo, useState, useMemo } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import mermaid from 'mermaid';
import { cn } from '@/lib/utils';
import { Check, Copy, Link2 } from 'lucide-react';

// =============================================================================
// Helper Functions
// =============================================================================

function slugify(text: any): string {
  if (typeof text !== 'string') {
    if (Array.isArray(text)) {
      return text.map(t => slugify(t)).join('');
    }
    if (text && typeof text === 'object' && text.props && text.props.children) {
      return slugify(text.props.children);
    }
    return '';
  }
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, '')
    .replace(/\s+/g, '-');
}

// =============================================================================
// Active Heading Hook
// =============================================================================

function useActiveHeading(headingIds: string[]) {
  const [activeId, setActiveId] = useState<string>('');

  useEffect(() => {
    if (headingIds.length === 0) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const visibleEntries = entries.filter((e) => e.isIntersecting);
        if (visibleEntries.length > 0) {
          // Highlight the first visible section heading
          setActiveId(visibleEntries[0].target.id);
        }
      },
      { rootMargin: '-80px 0px -60% 0px', threshold: 0.1 }
    );

    headingIds.forEach((id) => {
      const el = document.getElementById(id);
      if (el) observer.observe(el);
    });

    return () => {
      observer.disconnect();
    };
  }, [headingIds]);

  return activeId;
}

// =============================================================================
// Mermaid initialization (once)
// =============================================================================

let mermaidInitialized = false;

function initMermaid() {
  if (mermaidInitialized) return;
  mermaid.initialize({
    startOnLoad: false,
    theme: 'dark',
    themeVariables: {
      darkMode: true,
      background: 'transparent',
      primaryColor: '#4a3728',
      primaryTextColor: '#e8ddd3',
      primaryBorderColor: '#6b5744',
      lineColor: '#8a7560',
      secondaryColor: '#3a2a1e',
      tertiaryColor: '#2a1e14',
      fontFamily: 'var(--font-sans), system-ui, sans-serif',
    },
  });
  mermaidInitialized = true;
}

// =============================================================================
// Mermaid code block renderer
// =============================================================================

let mermaidIdCounter = 0;

const MermaidBlock = memo(function MermaidBlock({ code }: { code: string }) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    initMermaid();
    const id = `mermaid-${++mermaidIdCounter}`;

    mermaid
      .render(id, code)
      .then(({ svg }) => {
        if (containerRef.current) {
          containerRef.current.innerHTML = svg;
        }
      })
      .catch(() => {
        // Fallback to raw code on render failure
        if (containerRef.current) {
          containerRef.current.textContent = code;
          containerRef.current.className =
            'overflow-x-auto rounded-md bg-muted p-4 text-xs font-mono whitespace-pre';
        }
      });
  }, [code]);

  return (
    <div
      ref={containerRef}
      className="my-6 flex justify-center overflow-x-auto rounded-lg bg-muted/30 p-6"
    />
  );
});

// =============================================================================
// Copyable Code block component
// =============================================================================

const CodeBlock = memo(function CodeBlock({ children, className, ...props }: any) {
  const [copied, setCopied] = useState(false);
  const codeText = String(children).trim();

  const handleCopy = () => {
    navigator.clipboard.writeText(codeText);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="relative group/code my-4">
      <button
        onClick={handleCopy}
        className="absolute right-2 top-2 p-1.5 rounded-md border bg-card text-muted-foreground hover:text-foreground opacity-0 group-hover/code:opacity-100 transition-opacity duration-200 shadow-sm z-10"
        title="Copy to clipboard"
      >
        {copied ? <Check className="h-3.5 w-3.5 text-green-500" /> : <Copy className="h-3.5 w-3.5" />}
      </button>
      <code
        className="block overflow-x-auto rounded-lg bg-muted p-4 pr-12 text-xs font-mono leading-relaxed whitespace-pre"
        {...props}
      >
        {children}
      </code>
    </div>
  );
});

// =============================================================================
// Markdown prose styling — full-page optimized
// =============================================================================

const markdownComponents = {
  h1: ({ children, ...props }: any) => (
    <h1
      className="mb-4 mt-10 border-b border-border pb-2 text-2xl font-bold first:mt-0"
      {...props}
    >
      {children}
    </h1>
  ),
  h2: ({ children, ...props }: any) => {
    const id = slugify(children);
    return (
      <h2
        id={id}
        className="group/heading mb-3 mt-8 border-b border-border/50 pb-1.5 text-xl font-semibold scroll-mt-20 flex items-center justify-between lg:justify-start gap-2"
        {...props}
      >
        <span>{children}</span>
        {id && (
          <a
            href={`#${id}`}
            onClick={(e) => {
              e.preventDefault();
              document.getElementById(id)?.scrollIntoView({
                behavior: 'smooth',
                block: 'start',
              });
              window.history.pushState(null, '', `#${id}`);
              navigator.clipboard.writeText(window.location.origin + window.location.pathname + `#${id}`);
            }}
            className="opacity-0 group-hover/heading:opacity-100 transition-opacity duration-150 text-muted-foreground hover:text-foreground"
            title="Copy link to this section"
          >
            <Link2 className="h-4 w-4" />
          </a>
        )}
      </h2>
    );
  },
  h3: ({ children, ...props }: any) => {
    const id = slugify(children);
    return (
      <h3
        id={id}
        className="group/heading mb-2 mt-6 text-lg font-semibold scroll-mt-20 flex items-center justify-between lg:justify-start gap-2"
        {...props}
      >
        <span>{children}</span>
        {id && (
          <a
            href={`#${id}`}
            onClick={(e) => {
              e.preventDefault();
              document.getElementById(id)?.scrollIntoView({
                behavior: 'smooth',
                block: 'start',
              });
              window.history.pushState(null, '', `#${id}`);
              navigator.clipboard.writeText(window.location.origin + window.location.pathname + `#${id}`);
            }}
            className="opacity-0 group-hover/heading:opacity-100 transition-opacity duration-150 text-muted-foreground hover:text-foreground"
            title="Copy link to this section"
          >
            <Link2 className="h-3.5 w-3.5" />
          </a>
        )}
      </h3>
    );
  },
  h4: ({ children, ...props }: any) => (
    <h4 className="mb-2 mt-5 text-base font-semibold" {...props}>
      {children}
    </h4>
  ),

  p: ({ children, ...props }: any) => (
    <p className="mb-4 text-sm leading-relaxed text-foreground/90" {...props}>
      {children}
    </p>
  ),
  strong: ({ children, ...props }: any) => (
    <strong className="font-semibold text-foreground" {...props}>
      {children}
    </strong>
  ),
  em: ({ children, ...props }: any) => (
    <em className="text-foreground/80" {...props}>
      {children}
    </em>
  ),

  a: ({ children, href, ...props }: any) => (
    <a
      href={href}
      className="text-primary underline underline-offset-2 hover:text-primary/80"
      target="_blank"
      rel="noopener noreferrer"
      {...props}
    >
      {children}
    </a>
  ),

  ul: ({ children, ...props }: any) => (
    <ul className="mb-4 ml-6 list-disc space-y-1.5 text-sm" {...props}>
      {children}
    </ul>
  ),
  ol: ({ children, ...props }: any) => (
    <ol className="mb-4 ml-6 list-decimal space-y-1.5 text-sm" {...props}>
      {children}
    </ol>
  ),
  li: ({ children, ...props }: any) => (
    <li className="text-foreground/90 leading-relaxed" {...props}>
      {children}
    </li>
  ),

  table: ({ children, ...props }: any) => (
    <div className="mb-6 overflow-x-auto rounded-lg border">
      <table className="w-full text-sm" {...props}>
        {children}
      </table>
    </div>
  ),
  thead: ({ children, ...props }: any) => (
    <thead className="bg-muted/50" {...props}>
      {children}
    </thead>
  ),
  th: ({ children, ...props }: any) => (
    <th
      className="border-b px-4 py-2.5 text-left text-xs font-semibold text-muted-foreground"
      {...props}
    >
      {children}
    </th>
  ),
  td: ({ children, ...props }: any) => (
    <td className="border-b px-4 py-2.5 text-foreground/90" {...props}>
      {children}
    </td>
  ),
  tr: ({ children, ...props }: any) => (
    <tr className="last:border-b-0" {...props}>
      {children}
    </tr>
  ),

  code: ({ children, className, ...props }: any) => {
    const match = /language-(\w+)/.exec(className || '');
    const language = match ? match[1] : null;

    if (language === 'mermaid') {
      return <MermaidBlock code={String(children).trim()} />;
    }

    if (!className) {
      return (
        <code
          className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono text-foreground/90"
          {...props}
        >
          {children}
        </code>
      );
    }

    return <CodeBlock {...props} className={className}>{children}</CodeBlock>;
  },
  pre: ({ children, ...props }: any) => (
    <div className="mb-6" {...props}>
      {children}
    </div>
  ),

  blockquote: ({ children, ...props }: any) => (
    <blockquote
      className="mb-6 border-l-2 border-primary/50 pl-4 text-sm text-muted-foreground [&>p]:mb-1"
      {...props}
    >
      {children}
    </blockquote>
  ),

  hr: (props: any) => <hr className="my-8 border-border" {...props} />,
};

// =============================================================================
// Main Component — renders markdown in a full-page layout
// =============================================================================

interface WorkflowDocViewerProps {
  markdown: string;
}

interface TOCItem {
  id: string;
  text: string;
  level: number;
}

export function WorkflowDocViewer({ markdown }: WorkflowDocViewerProps) {
  // Extract headings
  const tocItems = useMemo(() => {
    const lines = markdown.split('\n');
    const items: TOCItem[] = [];
    const ids = new Set<string>();

    for (const line of lines) {
      const match = line.match(/^(#{2,3})\s+(.+)$/);
      if (match) {
        const level = match[1].length;
        const text = match[2].trim().replace(/[#*`|\-_>]/g, '');
        let id = slugify(text);
        if (id) {
          let suffix = 1;
          const baseId = id;
          while (ids.has(id)) {
            id = `${baseId}-${suffix++}`;
          }
          ids.add(id);
          items.push({ id, text, level });
        }
      }
    }
    return items;
  }, [markdown]);

  const headingIds = useMemo(() => tocItems.map((item) => item.id), [tocItems]);
  const activeId = useActiveHeading(headingIds);

  return (
    <div className="flex gap-8 relative items-start">
      {/* Article Content */}
      <article className="flex-1 max-w-none min-w-0 prose dark:prose-invert">
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={markdownComponents}
        >
          {markdown}
        </ReactMarkdown>
      </article>

      {/* Sticky Table of Contents */}
      {tocItems.length > 0 && (
        <aside className="hidden lg:block w-56 shrink-0 sticky top-24 self-start max-h-[calc(100vh-10rem)] overflow-y-auto pr-2 pb-4">
          <div className="space-y-3">
            <h4 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground/80">
              On This Page
            </h4>
            <ul className="space-y-2 text-xs border-l border-border/40 pl-0.5">
              {tocItems.map((item) => (
                <li
                  key={item.id}
                  style={{ paddingLeft: `${(item.level - 2) * 8}px` }}
                >
                  <a
                    href={`#${item.id}`}
                    onClick={(e) => {
                      e.preventDefault();
                      document.getElementById(item.id)?.scrollIntoView({
                        behavior: 'smooth',
                        block: 'start',
                      });
                      window.history.pushState(null, '', `#${item.id}`);
                    }}
                    className={cn(
                      'block py-0.5 transition-all duration-200 border-l pl-3 -ml-[1px]',
                      activeId === item.id
                        ? 'font-medium text-orange-600 dark:text-orange-400 border-orange-500/80 bg-orange-500/5 rounded-r'
                        : 'text-muted-foreground hover:text-foreground border-transparent'
                    )}
                  >
                    {item.text}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        </aside>
      )}
    </div>
  );
}
