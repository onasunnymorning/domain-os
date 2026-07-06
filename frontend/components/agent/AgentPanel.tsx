'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet';
import { ScrollArea } from '@/components/ui/scroll-area';
import { SystemBubble, UserBubble } from '@/components/shared/ChatBubbles';
import { AlpacaLogo } from '@/components/icons/AlpacaLogo';
import { DomainCard, type DomainCardData } from '@/components/agent/DomainCard';
import { EscalateCard } from '@/components/agent/EscalateCard';
import { TLDPricingCard, type TLDCardData } from '@/components/agent/TLDPricingCard';
import { EvidenceDrawer } from '@/components/agent/EvidenceDrawer';
import { AgentMarkdown } from '@/components/agent/AgentMarkdown';
import { askAgent, type AgentResult, type AgentSSEEvent } from '@/lib/api/agent';
import { cn } from '@/lib/utils';
import { Send, Loader2 } from 'lucide-react';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface ChatMessage {
  id: string;
  role: 'user' | 'agent';
  content: string;
  result?: AgentResult;
}

interface AgentPanelProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function findToolResult(evidence: AgentResult['evidence'], toolName: string): any | null {
  const entry = evidence?.find(
    (e) => e.tool?.toLowerCase().includes(toolName.toLowerCase()),
  );
  return entry?.result ?? null;
}

function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function AgentPanel({ open, onOpenChange }: AgentPanelProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const abortRef = useRef<AbortController | null>(null);

  // Auto-scroll to bottom when messages change
  useEffect(() => {
    if (scrollRef.current) {
      const viewport = scrollRef.current.querySelector('[data-slot="scroll-area-viewport"]');
      if (viewport) {
        viewport.scrollTop = viewport.scrollHeight;
      }
    }
  }, [messages, isLoading]);

  // Focus input when panel opens
  useEffect(() => {
    if (open) {
      setTimeout(() => inputRef.current?.focus(), 300);
    }
  }, [open]);

  // Cleanup abort controller on unmount
  useEffect(() => {
    return () => {
      abortRef.current?.abort();
    };
  }, []);

  const handleSubmit = useCallback(async () => {
    const question = input.trim();
    if (!question || isLoading) return;

    setInput('');
    const userMsg: ChatMessage = { id: generateId(), role: 'user', content: question };
    setMessages((prev) => [...prev, userMsg]);
    setIsLoading(true);

    // Abort any previous in-flight request
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      await askAgent(
        question,
        (event: AgentSSEEvent) => {
          if (event.type === 'result') {
            const result = event.data as AgentResult;
            const agentMsg: ChatMessage = {
              id: generateId(),
              role: 'agent',
              content: result.answer || result.reason || 'No response available.',
              result,
            };
            setMessages((prev) => [...prev, agentMsg]);
            setIsLoading(false);
          } else if (event.type === 'error') {
            const err = event.data as { error: string };
            const errorMsg: ChatMessage = {
              id: generateId(),
              role: 'agent',
              content: err.error,
            };
            setMessages((prev) => [...prev, errorMsg]);
            setIsLoading(false);
          }
        },
        controller.signal,
      );
    } catch (err: any) {
      if (err?.name !== 'AbortError') {
        const errorMsg: ChatMessage = {
          id: generateId(),
          role: 'agent',
          content: `Failed to reach agent: ${err?.message || 'Unknown error'}`,
        };
        setMessages((prev) => [...prev, errorMsg]);
      }
      setIsLoading(false);
    }
  }, [input, isLoading]);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="flex w-full flex-col sm:max-w-lg md:max-w-xl"
      >
        {/* Header */}
        <SheetHeader className="border-b border-border/50 pb-4">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-orange-500/20 to-amber-500/20 shadow-sm">
              <AlpacaLogo className="h-6 w-6" />
            </div>
            <div>
              <SheetTitle className="text-base">Ask Alpaca</SheetTitle>
              <SheetDescription className="text-xs">
                AI-powered registry assistant
              </SheetDescription>
            </div>
          </div>
        </SheetHeader>

        {/* Chat area */}
        <ScrollArea ref={scrollRef} className="min-h-0 flex-1 -mx-4 px-4">
          <div className="space-y-4 py-4">
            {/* Welcome message if no messages */}
            {messages.length === 0 && !isLoading && (
              <SystemBubble>
                <p>
                  Hi! I'm <span className="font-medium text-foreground">Alpaca</span>, your
                  registry assistant. Ask me about domains, TLD pricing, registrar info, or
                  anything else.
                </p>
              </SystemBubble>
            )}

            {/* Messages */}
            {messages.map((msg) => {
              if (msg.role === 'user') {
                return (
                  <UserBubble key={msg.id}>
                    {msg.content}
                  </UserBubble>
                );
              }

              // Agent message
              const result = msg.result;
              const isEscalation = result?.outcome === 'escalate';

              // Detect rich cards from evidence
              const domainData = result ? findToolResult(result.evidence, 'get_domain') : null;
              const tldData = result ? findToolResult(result.evidence, 'get_tld') : null;

              return (
                <div key={msg.id} className="space-y-3">
                  {/* Escalation: render only the card (it includes the reason) */}
                  {isEscalation ? (
                    <div className="animate-in fade-in slide-in-from-bottom-2 duration-300">
                      <EscalateCard reason={msg.content} />
                    </div>
                  ) : (
                    <SystemBubble>
                      <AgentMarkdown content={msg.content} />
                    </SystemBubble>
                  )}

                  {/* Rich domain card */}
                  {domainData && (
                    <div className="pl-10 animate-in fade-in slide-in-from-bottom-2 duration-300">
                      <DomainCard data={domainData as DomainCardData} />
                    </div>
                  )}

                  {/* Rich TLD card */}
                  {tldData && (
                    <div className="pl-10 animate-in fade-in slide-in-from-bottom-2 duration-300">
                      <TLDPricingCard data={tldData as TLDCardData} />
                    </div>
                  )}

                  {/* Evidence drawer */}
                  {result?.evidence && result.evidence.length > 0 && (
                    <div className="pl-10">
                      <EvidenceDrawer evidence={result.evidence} />
                    </div>
                  )}
                </div>
              );
            })}

            {/* Loading state */}
            {isLoading && (
              <SystemBubble>
                <div className="flex items-center gap-2 text-muted-foreground">
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span className="animate-pulse">Thinking...</span>
                </div>
              </SystemBubble>
            )}
          </div>
        </ScrollArea>

        {/* Input footer */}
        <div className="border-t border-border/50 pt-4">
          <div className="flex gap-2">
            <input
              ref={inputRef}
              type="text"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey && !isLoading) {
                  e.preventDefault();
                  handleSubmit();
                }
              }}
              placeholder="Ask about a domain, TLD, registrar..."
              disabled={isLoading}
              className="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
            />
            <button
              type="button"
              onClick={handleSubmit}
              disabled={isLoading || !input.trim()}
              className={cn(
                'inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-md transition-colors',
                'bg-primary text-primary-foreground hover:bg-primary/90',
                'disabled:opacity-50 disabled:pointer-events-none',
              )}
            >
              <Send className="h-4 w-4" />
            </button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
