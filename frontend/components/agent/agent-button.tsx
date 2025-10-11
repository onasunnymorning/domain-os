'use client';

import { useState, useEffect } from 'react';
import Image from 'next/image';
import { Button } from '@/components/ui/button';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { AgentChat } from './agent-chat';

export function AgentButton() {
  const [isOpen, setIsOpen] = useState(false);

  // Keyboard shortcut: Cmd+K / Ctrl+K
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setIsOpen(prev => !prev);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  return (
    <>
      <Button
        onClick={() => setIsOpen(true)}
        size="lg"
        className="fixed bottom-6 right-6 h-16 w-16 rounded-full shadow-lg p-0 bg-transparent hover:bg-transparent"
        title="Alpaca Agent (⌘K)"
      >
        <Image 
          src="/favicon.svg" 
          alt="Alpaca Agent" 
          width={64} 
          height={64}
        />
      </Button>

      <Sheet open={isOpen} onOpenChange={setIsOpen}>
        <SheetContent side="right" className="w-full sm:w-[540px] sm:max-w-[540px] p-0">
          <SheetHeader className="sr-only">
            <SheetTitle>Alpaca Agent</SheetTitle>
            <SheetDescription>
              Chat with the Alpaca Agent - your Domain-OS AI assistant
            </SheetDescription>
          </SheetHeader>
          <div className="h-full">
            <AgentChat onClose={() => setIsOpen(false)} />
          </div>
        </SheetContent>
      </Sheet>
    </>
  );
}
