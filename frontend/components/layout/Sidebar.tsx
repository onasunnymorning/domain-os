'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';
import {
  Building2,
  FileText,
  Home,
  Users,
  Globe,
  Server,
  ChevronLeft,
  Archive,
  BarChart,
  ServerOff,
  Zap
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { GRAFANA_URL } from '@/lib/constants/external-urls';

const navigation = [
  { name: 'Dashboard', href: '/', icon: Home },
  { name: 'Registry Operators', href: '/registry-operators', icon: Building2 },
  { name: 'TLDs', href: '/tlds', icon: Globe },
  { name: 'Registrars', href: '/registrars', icon: Users },
  { name: 'Domains', href: '/domains', icon: Server },
  { name: 'NNDNs', href: '/nndns', icon: ServerOff },
  { name: 'Workflows', href: '/workflows', icon: Zap },
  { name: 'Documentation', href: '/docs', icon: FileText },
  { name: 'TLD Imports', href: '/escrow', icon: Archive },
  { name: 'Analytics', href: GRAFANA_URL, icon: BarChart, target: '_blank' },
];

interface SidebarProps {
  isOpen?: boolean;
  onClose?: () => void;
}

export function Sidebar({ isOpen = true, onClose }: SidebarProps) {
  const pathname = usePathname();

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
          'fixed left-0 top-16 z-40 h-[calc(100vh-4rem)] w-64 bg-background transition-transform md:translate-x-0',
          isOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className="flex h-full flex-col gap-2 p-4">
          <nav className="flex-1 space-y-1">
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

          <div className="border-t pt-4">
            <p className="px-3 text-xs text-muted-foreground">
              v{process.env.NEXT_PUBLIC_APP_VERSION || '1.0.0'}
            </p>
          </div>
        </div>
      </aside>
    </>
  );
}
