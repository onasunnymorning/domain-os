'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { cn } from '@/lib/utils';

export interface StickySubNavSection {
  /** The DOM id of the section element to observe/scroll to */
  id: string;
  /** Display label in the nav */
  label: string;
  /** Optional count badge */
  count?: number;
}

interface StickySubNavProps {
  sections: StickySubNavSection[];
  /** Extra className on the outer wrapper */
  className?: string;
  /** Pixel offset for scroll-into-view, to account for sticky headers above. Default 80 */
  scrollOffset?: number;
}

/**
 * Horizontal sub-navigation bar that sticks below the page header.
 *
 * - Highlights the currently visible section using Intersection Observer
 * - Smooth-scrolls to a section when its nav item is clicked
 * - Completely reusable — pass any array of {id, label, count?} sections
 */
export function StickySubNav({
  sections,
  className,
  scrollOffset = 80,
}: StickySubNavProps) {
  const [activeId, setActiveId] = useState<string>(sections[0]?.id ?? '');
  const isClickScrolling = useRef(false);
  const clickTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // --- Intersection Observer ---
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        // During a programmatic click-scroll we ignore observer updates so the
        // clicked item stays highlighted until the scroll settles.
        if (isClickScrolling.current) return;

        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveId(entry.target.id);
          }
        }
      },
      {
        // rootMargin: negative top margin = sticky-nav height; bottom = –40% so
        // we trigger when the section is in the upper portion of the viewport.
        rootMargin: `-${scrollOffset + 16}px 0px -40% 0px`,
        threshold: 0,
      },
    );

    const elements = sections
      .map((s) => document.getElementById(s.id))
      .filter(Boolean) as HTMLElement[];

    elements.forEach((el) => observer.observe(el));

    return () => observer.disconnect();
  }, [sections, scrollOffset]);

  // --- Click handler ---
  const handleClick = useCallback(
    (id: string) => {
      const el = document.getElementById(id);
      if (!el) return;

      // Immediately mark as active
      setActiveId(id);

      // Suppress observer updates while the smooth scroll is in progress
      isClickScrolling.current = true;
      if (clickTimeoutRef.current) clearTimeout(clickTimeoutRef.current);
      clickTimeoutRef.current = setTimeout(() => {
        isClickScrolling.current = false;
      }, 900); // generous timeout for smooth scroll to finish

      const y = el.getBoundingClientRect().top + window.scrollY - scrollOffset;
      window.scrollTo({ top: y, behavior: 'smooth' });
    },
    [scrollOffset],
  );

  if (sections.length === 0) return null;

  return (
    <nav
      className={cn(
        'sticky top-0 z-30 -mx-6 px-6 bg-background/95 backdrop-blur-sm border-b border-border/60',
        className,
      )}
      aria-label="Page sections"
    >
      <ul className="flex items-center gap-1 overflow-x-auto scrollbar-none -mb-px">
        {sections.map((section) => {
          const isActive = activeId === section.id;
          return (
            <li key={section.id}>
              <button
                type="button"
                onClick={() => handleClick(section.id)}
                className={cn(
                  'relative inline-flex items-center gap-1.5 px-3 py-2.5 text-sm font-medium transition-colors whitespace-nowrap',
                  'hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 rounded-t-md',
                  isActive
                    ? 'text-primary'
                    : 'text-muted-foreground',
                )}
                aria-current={isActive ? 'true' : undefined}
              >
                {section.label}
                {section.count != null && (
                  <span
                    className={cn(
                      'inline-flex items-center justify-center rounded-full px-1.5 py-0.5 text-[10px] font-semibold leading-none',
                      isActive
                        ? 'bg-primary/15 text-primary'
                        : 'bg-muted text-muted-foreground',
                    )}
                  >
                    {section.count}
                  </span>
                )}
                {/* Active indicator — bottom border line */}
                {isActive && (
                  <span className="absolute inset-x-0 -bottom-px h-0.5 bg-primary rounded-full" />
                )}
              </button>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
