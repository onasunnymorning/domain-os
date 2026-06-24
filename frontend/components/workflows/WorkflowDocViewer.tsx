'use client';

import { useEffect, useRef, memo } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import mermaid from 'mermaid';

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
  h2: ({ children, ...props }: any) => (
    <h2
      className="mb-3 mt-8 border-b border-border/50 pb-1.5 text-xl font-semibold"
      {...props}
    >
      {children}
    </h2>
  ),
  h3: ({ children, ...props }: any) => (
    <h3 className="mb-2 mt-6 text-lg font-semibold" {...props}>
      {children}
    </h3>
  ),
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

    return (
      <code
        className="block overflow-x-auto rounded-lg bg-muted p-4 text-xs font-mono leading-relaxed whitespace-pre"
        {...props}
      >
        {children}
      </code>
    );
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

export function WorkflowDocViewer({ markdown }: WorkflowDocViewerProps) {
  return (
    <article className="max-w-none">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={markdownComponents}
      >
        {markdown}
      </ReactMarkdown>
    </article>
  );
}
