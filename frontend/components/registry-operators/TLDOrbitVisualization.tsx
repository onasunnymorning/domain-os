'use client';

import { useEffect, useRef, useCallback, useMemo } from 'react';
import { useRouter } from 'next/navigation';
import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCollide,
  forceX,
  forceY,
  type SimulationNodeDatum,
  type SimulationLinkDatum,
} from 'd3-force';
import type { RegistryOperator } from '@/lib/api/types';
import type { TLD } from '@/lib/api/tlds';
import { formatCompactNumber } from '@/lib/utils/numberUtils';
import { Skeleton } from '@/components/ui/skeleton';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface OrbitNode extends SimulationNodeDatum {
  id: string;
  label: string;
  sublabel?: string;
  radius: number;
  colorIndex: number;
  type: 'ro' | 'tld';
  href: string;
  domainCount?: number;
  parentRoId?: string;
}

interface OrbitLink extends SimulationLinkDatum<OrbitNode> {
  source: string | OrbitNode;
  target: string | OrbitNode;
}

// ---------------------------------------------------------------------------
// Color palette
// ---------------------------------------------------------------------------

const TLD_COLORS = [
  { fill: 'hsl(36, 90%, 55%)',  stroke: 'hsl(36, 90%, 45%)',  bg: 'hsl(36, 90%, 55%, 0.15)' },
  { fill: 'hsl(25, 90%, 55%)',  stroke: 'hsl(25, 90%, 45%)',  bg: 'hsl(25, 90%, 55%, 0.15)' },
  { fill: 'hsl(350, 70%, 55%)', stroke: 'hsl(350, 70%, 45%)', bg: 'hsl(350, 70%, 55%, 0.15)' },
  { fill: 'hsl(265, 55%, 58%)', stroke: 'hsl(265, 55%, 48%)', bg: 'hsl(265, 55%, 58%, 0.15)' },
  { fill: 'hsl(210, 65%, 55%)', stroke: 'hsl(210, 65%, 45%)', bg: 'hsl(210, 65%, 55%, 0.15)' },
  { fill: 'hsl(155, 60%, 45%)', stroke: 'hsl(155, 60%, 35%)', bg: 'hsl(155, 60%, 45%, 0.15)' },
  { fill: 'hsl(175, 55%, 45%)', stroke: 'hsl(175, 55%, 35%)', bg: 'hsl(175, 55%, 45%, 0.15)' },
  { fill: 'hsl(195, 65%, 50%)', stroke: 'hsl(195, 65%, 40%)', bg: 'hsl(195, 65%, 50%, 0.15)' },
];

const RO_COLORS = [
  { fill: 'hsl(30, 85%, 52%)',  stroke: 'hsl(30, 85%, 40%)' },
  { fill: 'hsl(340, 70%, 50%)', stroke: 'hsl(340, 70%, 38%)' },
  { fill: 'hsl(260, 60%, 55%)', stroke: 'hsl(260, 60%, 42%)' },
  { fill: 'hsl(200, 70%, 48%)', stroke: 'hsl(200, 70%, 36%)' },
  { fill: 'hsl(160, 65%, 42%)', stroke: 'hsl(160, 65%, 30%)' },
  { fill: 'hsl(45, 85%, 50%)',  stroke: 'hsl(45, 85%, 38%)' },
];

function getTLDColor(index: number) { return TLD_COLORS[index % TLD_COLORS.length]; }
function getROColor(index: number) { return RO_COLORS[index % RO_COLORS.length]; }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function computeTLDRadius(domainCount: number, maxCount: number): number {
  const MIN_R = 16;
  const MAX_R = 42;
  if (maxCount === 0) return MIN_R;
  return MIN_R + Math.sqrt(domainCount / maxCount) * (MAX_R - MIN_R);
}

function computeRORadius(tldCount: number): number {
  return Math.min(48, 28 + Math.sqrt(tldCount) * 5);
}

const CLICK_THRESHOLD = 5;

// ---------------------------------------------------------------------------
// Component — uses direct DOM manipulation for smooth 60fps animation
// ---------------------------------------------------------------------------

interface RegistryOrbitMapProps {
  operators: RegistryOperator[];
  tlds: TLD[];
  isLoading?: boolean;
}

export function RegistryOrbitMap({ operators, tlds, isLoading }: RegistryOrbitMapProps) {
  const router = useRouter();
  const svgRef = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const simulationRef = useRef<ReturnType<typeof forceSimulation<OrbitNode>> | null>(null);

  // DOM element ref maps — keyed by node/link id for direct manipulation
  const nodeGroupRefs = useRef<Map<string, SVGGElement>>(new Map());
  const linkRefs = useRef<Map<number, SVGLineElement>>(new Map());
  const glowRefs = useRef<Map<string, SVGCircleElement>>(new Map());

  // Mutable state for drag (not React state — no re-renders during drag)
  const dragRef = useRef<{
    nodeId: string;
    startX: number;
    startY: number;
    active: boolean;
  } | null>(null);

  const dimensionsRef = useRef({ width: 800, height: 550 });

  // ---------------------------------------------------------------------------
  // Build node/link data
  // ---------------------------------------------------------------------------
  const { nodeData, linkData } = useMemo(() => {
    const tldsByRo = new Map<string, TLD[]>();
    for (const tld of tlds) {
      const existing = tldsByRo.get(tld.RyID) || [];
      existing.push(tld);
      tldsByRo.set(tld.RyID, existing);
    }

    const maxCount = tlds.reduce((max, t) => Math.max(max, t.DomainCount ?? 0), 0);
    const allNodes: OrbitNode[] = [];
    const allLinks: OrbitLink[] = [];

    operators.forEach((op, roIndex) => {
      const roTlds = tldsByRo.get(op.RyID) || [];
      const roNode: OrbitNode = {
        id: `ro-${op.RyID}`,
        label: op.Name,
        radius: computeRORadius(roTlds.length),
        colorIndex: roIndex,
        type: 'ro',
        href: `/registry-operators/${op.RyID}`,
      };
      allNodes.push(roNode);

      const sorted = [...roTlds].sort((a, b) => (b.DomainCount ?? 0) - (a.DomainCount ?? 0));
      sorted.forEach((tld, tldIndex) => {
        const tldNode: OrbitNode = {
          id: `tld-${tld.Name}`,
          label: `.${tld.Name}`,
          sublabel: tld.DomainCount ? formatCompactNumber(tld.DomainCount) : undefined,
          radius: computeTLDRadius(tld.DomainCount ?? 0, maxCount),
          colorIndex: tldIndex,
          type: 'tld',
          href: `/tlds/${tld.Name}`,
          domainCount: tld.DomainCount ?? 0,
          parentRoId: `ro-${op.RyID}`,
        };
        allNodes.push(tldNode);
        allLinks.push({ source: roNode.id, target: tldNode.id });
      });
    });

    return { nodeData: allNodes, linkData: allLinks };
  }, [operators, tlds]);

  // ---------------------------------------------------------------------------
  // Responsive sizing
  // ---------------------------------------------------------------------------
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width } = entry.contentRect;
        const height = Math.max(400, Math.min(width * 0.6, 600));
        dimensionsRef.current = { width, height };

        // Update SVG viewBox directly
        const svg = svgRef.current;
        if (svg) svg.setAttribute('viewBox', `0 0 ${width} ${height}`);

        // Recenter forces if simulation is running
        const sim = simulationRef.current;
        if (sim) {
          sim.force('centerX', forceX<OrbitNode>(width / 2).strength(0.04));
          sim.force('centerY', forceY<OrbitNode>(height / 2).strength(0.04));
          sim.alpha(0.3).restart();
        }
      }
    });

    observer.observe(container);
    return () => observer.disconnect();
  }, []);

  // ---------------------------------------------------------------------------
  // Force simulation — updates DOM directly, no React re-renders
  // ---------------------------------------------------------------------------
  useEffect(() => {
    if (nodeData.length === 0) return;

    const { width: w, height: h } = dimensionsRef.current;
    const cx = w / 2;
    const cy = h / 2;

    // Clone nodes for d3 mutation
    const simNodes = nodeData.map((n) => ({ ...n }));
    const simLinks = linkData.map((l) => ({ ...l }));

    const roNodes = simNodes.filter((n) => n.type === 'ro');
    const roCount = roNodes.length;

    // Initial placement
    simNodes.forEach((node) => {
      if (node.type === 'ro') {
        const roIdx = roNodes.indexOf(node);
        if (roCount === 1) {
          node.x = cx;
          node.y = cy;
        } else {
          const angle = (roIdx / roCount) * 2 * Math.PI - Math.PI / 2;
          const spread = Math.min(cx, cy) * 0.45;
          node.x = cx + spread * Math.cos(angle);
          node.y = cy + spread * Math.sin(angle);
        }
      } else {
        const parentRo = simNodes.find((n) => n.id === node.parentRoId);
        if (parentRo) {
          const angle = Math.random() * 2 * Math.PI;
          const dist = 50 + Math.random() * 60;
          node.x = (parentRo.x ?? cx) + dist * Math.cos(angle);
          node.y = (parentRo.y ?? cy) + dist * Math.sin(angle);
        }
      }
    });

    const linkDistance = roCount <= 2 ? 100 : roCount <= 5 ? 80 : 60;

    const simulation = forceSimulation<OrbitNode>(simNodes)
      .force('link', forceLink<OrbitNode, OrbitLink>(simLinks)
        .id((d) => d.id)
        .distance(linkDistance)
        .strength(0.15)
      )
      .force('charge', forceManyBody<OrbitNode>()
        .strength((d) => d.type === 'ro' ? -300 : -30)
      )
      .force('collide', forceCollide<OrbitNode>()
        .radius((d) => d.radius + 5)
        .strength(0.85)
        .iterations(3)
      )
      .force('centerX', forceX<OrbitNode>(cx).strength(0.04))
      .force('centerY', forceY<OrbitNode>(cy).strength(0.04))
      .alphaDecay(0.01)
      .alphaMin(0.012)
      .velocityDecay(0.38)
      .on('tick', () => {
        const { width, height } = dimensionsRef.current;

        // Clamp positions
        for (const node of simNodes) {
          const pad = node.radius + 2;
          node.x = Math.max(pad, Math.min(width - pad, node.x!));
          node.y = Math.max(pad, Math.min(height - pad, node.y!));
        }

        // --- Direct DOM updates (no setState!) ---

        // Update link lines
        for (let i = 0; i < simLinks.length; i++) {
          const el = linkRefs.current.get(i);
          if (!el) continue;
          const s = simLinks[i].source as OrbitNode;
          const t = simLinks[i].target as OrbitNode;
          if (s.x == null || s.y == null || t.x == null || t.y == null) continue;
          el.setAttribute('x1', String(s.x));
          el.setAttribute('y1', String(s.y));
          el.setAttribute('x2', String(t.x));
          el.setAttribute('y2', String(t.y));
        }

        // Update node groups and glow circles
        for (const node of simNodes) {
          if (node.x == null || node.y == null) continue;

          const g = nodeGroupRefs.current.get(node.id);
          if (g) g.setAttribute('transform', `translate(${node.x}, ${node.y})`);

          const glow = glowRefs.current.get(node.id);
          if (glow) {
            glow.setAttribute('cx', String(node.x));
            glow.setAttribute('cy', String(node.y));
          }
        }
      });

    simulationRef.current = simulation;

    return () => {
      simulation.stop();
      simulationRef.current = null;
    };
  }, [nodeData, linkData]);

  // ---------------------------------------------------------------------------
  // Gentle periodic nudge
  // ---------------------------------------------------------------------------
  useEffect(() => {
    const interval = setInterval(() => {
      const sim = simulationRef.current;
      if (sim && sim.alpha() < 0.02) {
        sim.alpha(0.025).restart();
      }
    }, 5000);
    return () => clearInterval(interval);
  }, []);

  // ---------------------------------------------------------------------------
  // Pointer handlers — interaction via direct DOM + simulation mutation
  // ---------------------------------------------------------------------------
  const handlePointerDown = useCallback((e: React.PointerEvent, nodeId: string) => {
    e.preventDefault();
    e.stopPropagation();
    (e.target as SVGElement).setPointerCapture(e.pointerId);

    dragRef.current = { nodeId, startX: e.clientX, startY: e.clientY, active: false };

    const sim = simulationRef.current;
    if (sim) {
      sim.alphaTarget(0.12).restart();
      const node = sim.nodes().find((n) => n.id === nodeId);
      if (node) {
        node.fx = node.x;
        node.fy = node.y;
      }
    }

    // Visual feedback — apply drag style immediately
    const g = nodeGroupRefs.current.get(nodeId);
    if (g) g.style.filter = 'url(#drag-shadow)';
  }, []);

  const handlePointerMove = useCallback((e: React.PointerEvent) => {
    const drag = dragRef.current;
    if (!drag) return;

    const sim = simulationRef.current;
    const svg = svgRef.current;
    if (!sim || !svg) return;

    drag.active = true;

    const node = sim.nodes().find((n) => n.id === drag.nodeId);
    if (!node) return;

    const point = svg.createSVGPoint();
    point.x = e.clientX;
    point.y = e.clientY;
    const ctm = svg.getScreenCTM();
    if (!ctm) return;
    const svgPoint = point.matrixTransform(ctm.inverse());

    node.fx = svgPoint.x;
    node.fy = svgPoint.y;
  }, []);

  const handlePointerUp = useCallback((e: React.PointerEvent) => {
    const drag = dragRef.current;
    if (!drag) return;

    const distance = Math.sqrt(
      (e.clientX - drag.startX) ** 2 + (e.clientY - drag.startY) ** 2
    );

    const sim = simulationRef.current;
    if (sim) {
      sim.alphaTarget(0);
      const node = sim.nodes().find((n) => n.id === drag.nodeId);
      if (node) {
        node.fx = null;
        node.fy = null;
      }
    }

    // Remove drag visual style
    const g = nodeGroupRefs.current.get(drag.nodeId);
    if (g) g.style.filter = '';

    // Click → navigate
    if (distance < CLICK_THRESHOLD) {
      const target = nodeData.find((n) => n.id === drag.nodeId);
      if (target) router.push(target.href);
    }

    dragRef.current = null;
  }, [nodeData, router]);

  // Hover — CSS-class based, no React re-render
  const handlePointerEnter = useCallback((nodeId: string, parentRoId?: string) => {
    const roId = parentRoId ?? (nodeId.startsWith('ro-') ? nodeId : null);
    if (!roId) return;

    // Dim everything not in this cluster
    nodeGroupRefs.current.forEach((g, id) => {
      const node = nodeData.find((n) => n.id === id);
      if (!node) return;
      const inCluster = id === roId || node.parentRoId === roId;
      g.style.opacity = inCluster ? '1' : '0.2';
    });

    // Highlight links
    linkRefs.current.forEach((el) => {
      const parentId = el.dataset.sourceId;
      if (parentId === roId) {
        el.style.strokeOpacity = '0.5';
        el.style.strokeWidth = '2';
        el.style.strokeDasharray = 'none';
      } else {
        el.style.strokeOpacity = '0.08';
      }
    });
  }, [nodeData]);

  const handlePointerLeave = useCallback(() => {
    // Reset all opacities
    nodeGroupRefs.current.forEach((g) => {
      g.style.opacity = '';
    });
    linkRefs.current.forEach((el) => {
      el.style.strokeOpacity = '';
      el.style.strokeWidth = '';
      el.style.strokeDasharray = '';
    });
  }, []);

  // ---------------------------------------------------------------------------
  // Ref callbacks for collecting DOM elements
  // ---------------------------------------------------------------------------
  const setNodeRef = useCallback((id: string) => (el: SVGGElement | null) => {
    if (el) nodeGroupRefs.current.set(id, el);
    else nodeGroupRefs.current.delete(id);
  }, []);

  const setLinkRef = useCallback((index: number) => (el: SVGLineElement | null) => {
    if (el) linkRefs.current.set(index, el);
    else linkRefs.current.delete(index);
  }, []);

  const setGlowRef = useCallback((id: string) => (el: SVGCircleElement | null) => {
    if (el) glowRefs.current.set(id, el);
    else glowRefs.current.delete(id);
  }, []);

  // ---------------------------------------------------------------------------
  // Loading state
  // ---------------------------------------------------------------------------
  if (isLoading) {
    return (
      <div className="flex items-center justify-center" style={{ minHeight: 400 }}>
        <div className="relative">
          <Skeleton className="h-16 w-16 rounded-full" />
          <Skeleton className="absolute -top-6 right-[-2.5rem] h-10 w-10 rounded-full" />
          <Skeleton className="absolute -bottom-4 left-[-3rem] h-12 w-12 rounded-full" />
          <Skeleton className="absolute top-[-2rem] left-12 h-8 w-8 rounded-full" />
          <Skeleton className="absolute bottom-[-1.5rem] right-[-1rem] h-14 w-14 rounded-full" />
        </div>
      </div>
    );
  }

  if (operators.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
        No registry operators to display
      </div>
    );
  }

  const { width, height } = dimensionsRef.current;

  // ---------------------------------------------------------------------------
  // Render — static structure; positions updated by simulation tick directly
  // ---------------------------------------------------------------------------
  return (
    <div
      ref={containerRef}
      className="relative w-full select-none"
      style={{ minHeight: 400 }}
    >
      <svg
        ref={svgRef}
        viewBox={`0 0 ${width} ${height}`}
        className="w-full h-auto"
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerLeave={(e) => { handlePointerUp(e); handlePointerLeave(); }}
      >
        <defs>
          {RO_COLORS.map((color, i) => (
            <radialGradient key={`ro-glow-${i}`} id={`ro-glow-${i}`} cx="50%" cy="50%" r="50%">
              <stop offset="0%" stopColor={color.fill} stopOpacity="0.25" />
              <stop offset="100%" stopColor={color.fill} stopOpacity="0" />
            </radialGradient>
          ))}
          <filter id="node-shadow" x="-30%" y="-30%" width="160%" height="160%">
            <feDropShadow dx="0" dy="2" stdDeviation="3" floodOpacity="0.12" floodColor="hsl(30, 60%, 30%)" />
          </filter>
          <filter id="drag-shadow" x="-30%" y="-30%" width="160%" height="160%">
            <feDropShadow dx="0" dy="4" stdDeviation="6" floodOpacity="0.2" floodColor="hsl(30, 60%, 30%)" />
          </filter>
        </defs>

        {/* Spoke lines — rendered once, positions updated by tick */}
        {linkData.map((link, i) => {
          const sourceId = typeof link.source === 'string' ? link.source : link.source.id;
          return (
            <line
              key={`link-${i}`}
              ref={setLinkRef(i)}
              data-source-id={sourceId}
              stroke="currentColor"
              className="text-muted-foreground"
              strokeOpacity={0.2}
              strokeWidth={1.25}
              strokeDasharray="4 4"
              style={{ transition: 'stroke-opacity 0.2s, stroke-width 0.2s, stroke-dasharray 0.2s' }}
            />
          );
        })}

        {/* RO glow backgrounds */}
        {nodeData.filter((n) => n.type === 'ro').map((node) => (
          <circle
            key={`glow-${node.id}`}
            ref={setGlowRef(node.id)}
            r={node.radius * 2}
            fill={`url(#ro-glow-${node.colorIndex % RO_COLORS.length})`}
          />
        ))}

        {/* All nodes */}
        {nodeData.map((node) => {
          const isRO = node.type === 'ro';

          if (isRO) {
            const roColor = getROColor(node.colorIndex);
            return (
              <g
                key={node.id}
                ref={setNodeRef(node.id)}
                style={{ cursor: 'pointer', transition: 'opacity 0.2s' }}
                filter="url(#node-shadow)"
                onPointerDown={(e) => handlePointerDown(e, node.id)}
                onPointerEnter={() => handlePointerEnter(node.id)}
                onPointerLeave={handlePointerLeave}
              >
                <circle
                  r={node.radius}
                  fill={roColor.fill}
                  stroke={roColor.stroke}
                  strokeWidth={2}
                  opacity={0.95}
                />
                <text
                  textAnchor="middle"
                  dominantBaseline="central"
                  fill="white"
                  fontSize={node.radius > 36 ? 11 : 9}
                  fontWeight={700}
                  fontFamily="var(--font-sans), Inter, system-ui, sans-serif"
                  style={{ pointerEvents: 'none' }}
                >
                  {node.label.length > 14 ? node.label.slice(0, 12) + '…' : node.label}
                </text>
              </g>
            );
          }

          // TLD node
          const color = getTLDColor(node.colorIndex);
          return (
            <g
              key={node.id}
              ref={setNodeRef(node.id)}
              style={{ cursor: 'pointer', transition: 'opacity 0.2s' }}
              filter="url(#node-shadow)"
              onPointerDown={(e) => handlePointerDown(e, node.id)}
              onPointerEnter={() => handlePointerEnter(node.id, node.parentRoId)}
              onPointerLeave={handlePointerLeave}
            >
              <circle
                r={node.radius}
                fill={color.bg}
                stroke={color.stroke}
                strokeWidth={1.5}
                strokeOpacity={0.35}
              />
              <circle
                r={node.radius * 0.82}
                fill={color.fill}
                opacity={0.1}
              />
              <text
                textAnchor="middle"
                dominantBaseline={node.sublabel && node.radius > 22 ? 'auto' : 'central'}
                y={node.sublabel && node.radius > 22 ? -2 : 0}
                fill="currentColor"
                className="text-foreground"
                fontSize={node.radius > 28 ? 12 : node.radius > 20 ? 10 : 8}
                fontWeight={600}
                fontFamily="var(--font-sans), Inter, system-ui, sans-serif"
                style={{ pointerEvents: 'none' }}
              >
                {node.label}
              </text>
              {node.sublabel && node.radius > 22 && (
                <text
                  textAnchor="middle"
                  dominantBaseline="hanging"
                  y={4}
                  fill="currentColor"
                  className="text-muted-foreground"
                  fontSize={node.radius > 32 ? 9 : 7}
                  fontWeight={400}
                  fontFamily="var(--font-console), IBM Plex Mono, monospace"
                  opacity={0.65}
                  style={{ pointerEvents: 'none' }}
                >
                  {node.sublabel}
                </text>
              )}
            </g>
          );
        })}
      </svg>
    </div>
  );
}
