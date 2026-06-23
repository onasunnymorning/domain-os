'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { Menu, Search, ExternalLink, Clock, Database } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { AlpacaLogo } from '@/components/icons/AlpacaLogo';
import { UserMenu } from '@/components/auth/user-menu';
import { GlobalSearch } from '@/components/search/GlobalSearch';

interface HeaderProps {
  onMenuClick?: () => void;
}

export function Header({ onMenuClick }: HeaderProps) {
  const [searchOpen, setSearchOpen] = useState(false);

  // Global ⌘K / Ctrl+K shortcut
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setSearchOpen((prev) => !prev);
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, []);

  return (
    <>
      <header className="sticky top-0 z-40 bg-background">
        <div className="flex h-16 items-center">
          {/* Left section — matches sidebar width */}
          <div className="flex h-full w-64 shrink-0 items-center gap-4 px-6">
            <Button
              variant="ghost"
              size="icon"
              className="md:hidden"
              onClick={onMenuClick}
            >
              <Menu className="h-5 w-5" />
            </Button>

            <Link href="/" className="flex items-center gap-2 font-semibold">
              <AlpacaLogo className="h-10 w-10" />
              <span className="hidden sm:inline-block">Alpaca Names</span>
            </Link>
          </div>

          {/* Right section — aligns with main content */}
          <div className="flex flex-1 items-center gap-4 px-6">
            {/* Search trigger */}
            <button
              onClick={() => setSearchOpen(true)}
              className="hidden items-center gap-2 rounded-md border bg-muted/40 px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground sm:flex"
            >
              <Search className="h-3.5 w-3.5" />
              <span>Search...</span>
              <kbd className="pointer-events-none ml-4 hidden select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground md:inline-flex">
                <span className="text-xs">⌘</span>K
              </kbd>
            </button>

            {/* External tool links */}
            <div className="hidden items-center gap-1 sm:flex">
              <a
                href="http://localhost:8081"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                title="Temporal UI"
              >
                <Clock className="h-3.5 w-3.5" />
                <span className="hidden lg:inline">Temporal</span>
                <ExternalLink className="h-3 w-3 opacity-50" />
              </a>
              <a
                href="http://localhost:9001"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                title="Storage"
              >
                <Database className="h-3.5 w-3.5" />
                <span className="hidden lg:inline">Storage</span>
                <ExternalLink className="h-3 w-3 opacity-50" />
              </a>
            </div>

            {/* Mobile search button */}
            <Button
              variant="ghost"
              size="icon"
              className="sm:hidden"
              onClick={() => setSearchOpen(true)}
            >
              <Search className="h-5 w-5" />
            </Button>

            <div className="ml-auto flex items-center gap-4">
              <UserMenu />
            </div>
          </div>
        </div>
      </header>

      <GlobalSearch open={searchOpen} onOpenChange={setSearchOpen} />
    </>
  );
}
