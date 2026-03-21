import { Node, Edge } from '@xyflow/react';

export interface DnsVizData {
  [domain: string]: any;
}

export function mapDNSVizToReactFlow(data: DnsVizData): { nodes: Node[]; edges: Edge[] } {
  const rfNodes: Node[] = [];
  const rfEdges: Edge[] = [];

  if (!data || Array.isArray(data)) {
    return { nodes: [], edges: [] };
  }

  // Extract domains and sort by length to guarantee hierarchy: '.' -> 'online.' -> 'nic.online.'
  const domains = Object.keys(data)
    .filter(k => typeof k === 'string' && (k.endsWith('.') || k === '.'))
    .sort((a, b) => a.length - b.length);

  function mapStatus(s: string): string {
    if (!s) return 'unknown';
    const u = s.toUpperCase();
    if (u.includes('BOGUS') || u.includes('INVALID') || u.includes('ERROR')) return 'bogus';
    if (u.includes('SECURE') || u.includes('VALID') || u.includes('NOERROR')) return 'secure';
    if (u.includes('INSECURE') || u.includes('WARNING') || u.includes('INDETERMINATE')) return 'warning';
    return 'unknown';
  }

  function addEdge(source: string, target: string, color: string, dashed: boolean = false) {
    if (!source || !target) return;
    rfEdges.push({
      id: `edge-${source}-${target}`,
      source,
      target,
      animated: color !== '#ef4444', // animate unless broken/red
      style: { stroke: color, strokeWidth: 2, strokeDasharray: dashed ? '5,5' : 'none' },
      data: { raw: {} }
    });
  }

  domains.forEach((k, idx) => {
    const domainData = data[k];
    const Y_BLOCK = idx * 600;
    const X_CENTER = 400;

    const dsList = domainData.delegation?.ds || [];
    const keyList = domainData.dnskey || [];

    // flags 257 = KSK, flags 256 = ZSK (or 0 often means ZSK implicitly)
    const kskList = keyList.filter((key: any) => key.flags === 257);
    const zskList = keyList.filter((key: any) => key.flags === 256 || key.flags === 0);

    const parentDomainId = idx > 0 ? domains[idx - 1] : null;

    // 1. Process DS Records and Parent Connections
    if (idx > 0 && parentDomainId) {
      if (dsList.length > 0) {
        dsList.forEach((ds: any, dsIdx: number) => {
          const dsId = `${k}-ds-${ds.id}`;
          const dsStatus = mapStatus(ds.status || domainData.delegation?.status);
          
          rfNodes.push({
            id: dsId,
            position: { x: X_CENTER - ((dsList.length - 1) * 220) / 2 + dsIdx * 220, y: Y_BLOCK },
            data: { label: `[DS] ${ds.id.split('/').slice(1).join('/')}`, status: dsStatus, raw: ds },
            type: 'terminalNode'
          });

          // Connect Parent Domain to DS
          addEdge(parentDomainId, dsId, '#6b7280'); // standard connection

          // Connect DS to matching KSK
          const matchKsk = kskList.find((key: any) => ds.id.startsWith(key.id + '/'));
          if (matchKsk) {
            const kskStatus = mapStatus(matchKsk.status || 'secure');
            addEdge(dsId, `${k}-ksk-${matchKsk.id}`, kskStatus === 'bogus' || dsStatus === 'bogus' ? '#ef4444' : '#10b981');
          } else {
            // Missing KSK for this DS
            addEdge(dsId, k, '#ef4444', true);
          }
        });
      } else {
        // No DS records! Connect parent directly to Domain or KSKs marking it insecure
        if (kskList.length > 0) {
          kskList.forEach((ksk: any) => addEdge(parentDomainId, `${k}-ksk-${ksk.id}`, '#eab308', true));
        } else {
          addEdge(parentDomainId, k, '#eab308', true); // Insecure delegation
        }
      }
    }

    // 2. Process KSKs
    kskList.forEach((ksk: any, kskIdx: number) => {
      const kskId = `${k}-ksk-${ksk.id}`;
      rfNodes.push({
        id: kskId,
        position: { x: X_CENTER - ((kskList.length - 1) * 220) / 2 + kskIdx * 220, y: Y_BLOCK + 150 },
        data: { label: `[KSK] ${ksk.id.split('/').slice(1).join('/')}`, status: mapStatus(ksk.status || 'secure'), raw: ksk },
        type: 'terminalNode'
      });

      // Connect KSK to ZSKs, or directly to domain if no ZSKs
      if (zskList.length > 0) {
        zskList.forEach((zsk: any) => {
          addEdge(kskId, `${k}-zsk-${zsk.id}`, '#10b981');
        });
      } else {
        addEdge(kskId, k, '#10b981');
      }
    });

    // 3. Process ZSKs
    zskList.forEach((zsk: any, zskIdx: number) => {
      const zskId = `${k}-zsk-${zsk.id}`;
      rfNodes.push({
        id: zskId,
        position: { x: X_CENTER - ((zskList.length - 1) * 220) / 2 + zskIdx * 220, y: Y_BLOCK + 300 },
        data: { label: `[ZSK] ${zsk.id.split('/').slice(1).join('/')}`, status: mapStatus(zsk.status || 'secure'), raw: zsk },
        type: 'terminalNode'
      });

      // Connect ZSK to Domain
      addEdge(zskId, k, '#10b981');
    });

    // 4. Process Domain Node
    const mainStatus = domainData.status || '';
    const delStatus = domainData.delegation?.status || '';
    let domainStatus = 'unknown';

    if (mainStatus.includes('BOGUS') || delStatus.includes('BOGUS')) {
      domainStatus = 'bogus';
    } else if (mainStatus.includes('SECURE') || delStatus.includes('SECURE')) {
      domainStatus = 'secure';
    } else if (mainStatus.includes('INSECURE') || delStatus.includes('INSECURE')) {
      domainStatus = 'insecure';
    } else if (mainStatus.includes('NOERROR')) {
      domainStatus = 'valid';
    }

    // Determine Y position for domain node
    // Make sure we space out correctly if root because root doesn't have DS at Y_BLOCK
    const domainY = Y_BLOCK + 450;

    rfNodes.push({
      id: k,
      position: { x: X_CENTER, y: domainY },
      data: {
        label: k === '.' ? 'ROOT (.)' : k,
        status: domainStatus,
        raw: domainData
      },
      type: 'terminalNode' // Assuming this matches the existing standard node type
    });
  });

  return { nodes: rfNodes, edges: rfEdges };
}
