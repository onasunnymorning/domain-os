'use client';

import Link from 'next/link';
import { Menu, User } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { AlpacaLogo } from '@/components/icons/AlpacaLogo';

interface HeaderProps {
  onMenuClick?: () => void;
}

export function Header({ onMenuClick }: HeaderProps) {
  return (
    <header className="sticky top-0 z-40 border-b bg-background">
      <div className="flex h-16 items-center gap-4 px-6">
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

        <div className="ml-auto flex items-center gap-4">
          <div className="flex items-center gap-2">
            <User className="h-4 w-4" />
            <span className="text-sm text-muted-foreground">Admin User</span>
          </div>
        </div>
      </div>
    </header>
  );
}
