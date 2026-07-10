'use client';

import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { cn } from '@/lib/utils';

/**
 * AgentMarkdown — lightweight markdown renderer for agent chat responses.
 * Supports bold, italic, lists, links, code, and tables via GFM.
 * Intentionally minimal compared to WorkflowDocViewer (no ToC, mermaid, etc.).
 */
export function AgentMarkdown({ content, className }: { content: string; className?: string }) {
  return (
    <div className={cn('agent-markdown', className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          // Headings — keep compact for chat
          h1: ({ children }) => (
            <h3 className="mt-3 mb-1.5 text-sm font-semibold text-foreground">{children}</h3>
          ),
          h2: ({ children }) => (
            <h4 className="mt-2.5 mb-1 text-sm font-semibold text-foreground">{children}</h4>
          ),
          h3: ({ children }) => (
            <h5 className="mt-2 mb-1 text-xs font-semibold text-foreground">{children}</h5>
          ),
          // Paragraphs
          p: ({ children }) => (
            <p className="mb-2 last:mb-0 leading-relaxed">{children}</p>
          ),
          // Lists
          ul: ({ children }) => (
            <ul className="mb-2 ml-4 list-disc space-y-0.5 last:mb-0">{children}</ul>
          ),
          ol: ({ children }) => (
            <ol className="mb-2 ml-4 list-decimal space-y-0.5 last:mb-0">{children}</ol>
          ),
          li: ({ children }) => (
            <li className="leading-relaxed">{children}</li>
          ),
          // Inline code
          code: ({ children, className: codeClassName }) => {
            const isBlock = codeClassName?.startsWith('language-');
            if (isBlock) {
              return (
                <pre className="my-2 overflow-x-auto rounded-md bg-muted/50 p-2 text-[11px] font-mono leading-relaxed">
                  <code>{children}</code>
                </pre>
              );
            }
            return (
              <code className="rounded bg-muted/50 px-1 py-0.5 text-[11px] font-mono">
                {children}
              </code>
            );
          },
          // Block code
          pre: ({ children }) => <>{children}</>,
          // Strong / emphasis
          strong: ({ children }) => (
            <strong className="font-semibold text-foreground">{children}</strong>
          ),
          em: ({ children }) => (
            <em className="italic">{children}</em>
          ),
          // Links
          a: ({ href, children }) => (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary underline underline-offset-2 hover:text-primary/80 transition-colors"
            >
              {children}
            </a>
          ),
          // Horizontal rule
          hr: () => <hr className="my-3 border-border/40" />,
          // Tables (GFM)
          table: ({ children }) => (
            <div className="my-2 overflow-x-auto rounded-md border border-border/40">
              <table className="w-full text-[11px]">{children}</table>
            </div>
          ),
          thead: ({ children }) => (
            <thead className="bg-muted/30">{children}</thead>
          ),
          th: ({ children }) => (
            <th className="px-2 py-1 text-left font-medium text-muted-foreground">{children}</th>
          ),
          td: ({ children }) => (
            <td className="px-2 py-1 border-t border-border/30">{children}</td>
          ),
          // Blockquotes
          blockquote: ({ children }) => (
            <blockquote className="my-2 border-l-2 border-primary/30 pl-3 text-muted-foreground italic">
              {children}
            </blockquote>
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
