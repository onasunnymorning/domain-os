'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useCallback, useState } from 'react';
import { cn } from '@/lib/utils';
import {
  Building2,
  FileText,
  Home,
  Users,
  Globe,
  Server,
  ChevronLeft,
  ServerOff,
  Zap,
  Cloud
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { AlpacaLogo } from '@/components/icons/AlpacaLogo';
import { CoLogo } from '@/components/icons/CoLogo';
const navigation = [
  { name: 'Dashboard', href: '/', icon: Home },
  { name: 'Registry Operators', href: '/registry-operators', icon: Building2 },
  { name: 'TLDs', href: '/tlds', icon: Globe },
  { name: 'Registrars', href: '/registrars', icon: Users },
  { name: 'Domains', href: '/domains', icon: Server },
  { name: 'Blocking', href: '/nndns', icon: ServerOff },
  { name: 'Workflows', href: '/workflows', icon: Zap },
  { name: 'Cloud', href: '/cloud', icon: Cloud },
  { name: 'Documentation', href: '/docs', icon: FileText },
];

interface SidebarProps {
  isOpen?: boolean;
  onClose?: () => void;
}

export function Sidebar({ isOpen = true, onClose }: SidebarProps) {
  const pathname = usePathname();
  const [showAlpaca, setShowAlpaca] = useState(false);

  const toggleBrand = useCallback(() => {
    setShowAlpaca((prev) => {
      const next = !prev;
      // Swap tab title
      document.title = next ? 'Alpaca Names' : 'CO Registry';
      // Swap favicon
      const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
      if (link) {
        link.href = next ? '/favicon.svg' : '/favicon-co.svg';
      }
      return next;
    });
  }, []);

  return (
    <>
      {/* Mobile overlay */}
      {isOpen && (
        <div
          className="fixed inset-0 z-40 bg-background/80 backdrop-blur-sm md:hidden"
          onClick={onClose}
        />
      )}

      {/* Sidebar */}
      <aside
        className={cn(
          'fixed left-0 top-0 z-50 h-screen w-64 bg-background transition-transform md:translate-x-0',
          isOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className="flex h-full flex-col gap-4 pb-4">
          {/* Logo container */}
          <div className="flex h-32 items-center justify-center px-6 flex-col gap-1">
            <Link href="/" onClick={onClose} className="flex items-center justify-center">
              <div className="relative h-20 w-20">
                <div
                  className={cn(
                    'absolute inset-0 transition-all duration-500 ease-in-out',
                    showAlpaca
                      ? 'opacity-0 scale-90 rotate-12'
                      : 'opacity-100 scale-100 rotate-0'
                  )}
                >
                  <CoLogo className="h-20 w-20 text-[#DBFF00]" />
                </div>
                <div
                  className={cn(
                    'absolute inset-0 transition-all duration-500 ease-in-out',
                    showAlpaca
                      ? 'opacity-100 scale-100 rotate-0'
                      : 'opacity-0 scale-90 -rotate-12'
                  )}
                >
                  <AlpacaLogo className="h-20 w-20" />
                </div>
              </div>
            </Link>
            {/* Subtle logo switcher */}
            <button
              onClick={toggleBrand}
              className="group flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] text-muted-foreground/40 hover:text-muted-foreground/70 transition-colors duration-200 select-none"
              title={showAlpaca ? 'Switch to .co' : 'Switch to Alpaca'}
              aria-label="Toggle logo"
            >
              <span
                className={cn(
                  'inline-block w-1 h-1 rounded-full transition-colors duration-300',
                  showAlpaca ? 'bg-muted-foreground/30' : 'bg-[#DBFF00]'
                )}
              />
              <span
                className={cn(
                  'inline-block w-1 h-1 rounded-full transition-colors duration-300',
                  showAlpaca ? 'bg-orange-400' : 'bg-muted-foreground/30'
                )}
              />
            </button>
          </div>

          <nav className="flex-1 space-y-1 px-4 overflow-y-auto">
            {navigation.map((item) => {
              const isActive = item.href === '/'
                ? pathname === '/'
                : pathname.startsWith(item.href);
              return (
                <Link
                  key={item.name}
                  href={item.href}
                  onClick={onClose}
                  target={(item as any).target}
                  className={cn(
                    'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                    isActive
                      ? 'bg-primary text-primary-foreground'
                      : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                  )}
                >
                  <item.icon className="h-4 w-4" />
                  {item.name}
                </Link>
              );
            })}
          </nav>

          <div className="pt-4 px-4">
            <p className="px-3 text-xs text-muted-foreground">
              v{process.env.NEXT_PUBLIC_APP_VERSION || '1.0.0'}
            </p>
          </div>
        </div>
      </aside>
    </>
  );
}
