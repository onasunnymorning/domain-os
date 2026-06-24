'use client';

import { useTheme } from 'next-themes';
import { useEffect, useState } from 'react';
import { Moon, Monitor } from 'lucide-react';
import { SolDeMayo } from '@/components/icons/SolDeMayo';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';

export function ThemeSwitcher() {
  const { theme, setTheme, resolvedTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  // Avoid hydration mismatch — only render after mount
  useEffect(() => setMounted(true), []);

  if (!mounted) {
    return (
      <Button variant="ghost" size="icon" className="h-9 w-9" disabled>
        <div className="h-4 w-4" />
      </Button>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="h-9 w-9 relative"
          aria-label="Toggle theme"
        >
          {/* Sol de Mayo for light mode — visible when light */}
          <SolDeMayo
            className={cn(
              'h-5 w-5 text-orange-500 transition-all duration-500',
              resolvedTheme === 'dark'
                ? 'scale-0 rotate-90 opacity-0'
                : 'scale-100 rotate-0 opacity-100',
              'absolute',
            )}
          />
          {/* Moon for dark mode — visible when dark */}
          <Moon
            className={cn(
              'h-4 w-4 text-amber-300 transition-all duration-500',
              resolvedTheme === 'dark'
                ? 'scale-100 rotate-0 opacity-100'
                : 'scale-0 -rotate-90 opacity-0',
              'absolute',
            )}
          />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-[140px]">
        <DropdownMenuItem
          onClick={() => setTheme('light')}
          className={cn('gap-2', theme === 'light' && 'font-medium')}
        >
          <SolDeMayo className="h-4 w-4 text-orange-500" />
          Light
          {theme === 'light' && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => setTheme('dark')}
          className={cn('gap-2', theme === 'dark' && 'font-medium')}
        >
          <Moon className="h-4 w-4" />
          Dark
          {theme === 'dark' && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => setTheme('system')}
          className={cn('gap-2', theme === 'system' && 'font-medium')}
        >
          <Monitor className="h-4 w-4" />
          System
          {theme === 'system' && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
