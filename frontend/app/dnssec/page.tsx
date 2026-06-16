'use client';

import React, { useState, useEffect, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import { apiClient } from '../../lib/api/client';
import { DnssecGraph } from '../../components/dnssec/DnssecGraph';
import { mapDNSVizToReactFlow } from '../../lib/dnssec';
import { Node, Edge } from '@xyflow/react';

function DnssecContent() {
  const searchParams = useSearchParams();
  const initialDomain = searchParams.get('domain') || '';

  const [domain, setDomain] = useState(initialDomain);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [graphData, setGraphData] = useState<{ nodes: Node[]; edges: Edge[] } | null>(null);
  const [selectedNode, setSelectedNode] = useState<any | null>(null);

  // Auto-submit on mount if a domain is provided
  useEffect(() => {
    if (initialDomain) {
      handleSearch(new Event('submit') as unknown as React.FormEvent);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!domain.trim()) return;

    setLoading(true);
    setError(null);
    setGraphData(null);
    setSelectedNode(null);

    try {
      const res = await apiClient.get(`/api/v1/dnssec?domain=${encodeURIComponent(domain)}`);
      const data = res.data;
      
      const mapped = mapDNSVizToReactFlow(data);
      if (mapped.nodes.length === 0) {
         setError("Warning: DNSViz returned no nodes. The domain might not exist or lacks DNSSEC config.");
      }
      setGraphData(mapped);

    } catch (err: any) {
      const msg = err.response?.data?.error || err.response?.data?.message || err.message || 'An unknown error occurred';
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex flex-col h-screen bg-gray-950 text-gray-100 font-mono">
      {/* Header */}
      <header className="p-4 border-b border-gray-800 flex items-center justify-between shadow-md z-10 bg-gray-900">
        <div>
          <h1 className="text-xl font-bold text-emerald-400">DNSSEC Visualizer</h1>
          <p className="text-xs text-gray-400">Powered by dnsviz & React Flow</p>
        </div>
        <form onSubmit={handleSearch} className="flex gap-2">
          <input
            type="text"
            className="px-3 py-1 bg-black border border-gray-700 rounded text-sm focus:outline-none focus:border-emerald-500 w-64"
            placeholder="example.com"
            value={domain}
            onChange={(e) => setDomain(e.target.value)}
            disabled={loading}
          />
          <button
            type="submit"
            disabled={loading || !domain}
            className="px-4 py-1 bg-emerald-600 hover:bg-emerald-500 text-white font-bold rounded text-sm disabled:opacity-50 transition-colors"
          >
            {loading ? 'Analyzing...' : 'Analyze'}
          </button>
        </form>
      </header>

      {/* Main Content */}
      <main className="flex-1 flex overflow-hidden relative">
        {/* Error Terminal overlay */}
        {error && (
          <div className="absolute inset-0 z-20 flex items-center justify-center bg-black/80 backdrop-blur-sm p-8">
            <div className="w-full max-w-3xl bg-black border border-red-500 rounded p-4 shadow-2xl">
              <div className="flex items-center gap-2 mb-4 border-b border-red-900 pb-2">
                <div className="w-3 h-3 rounded-full bg-red-500" />
                <div className="w-3 h-3 rounded-full bg-yellow-500" />
                <div className="w-3 h-3 rounded-full bg-green-500" />
                <span className="text-red-500 text-sm font-bold ml-2">Terminal Error</span>
              </div>
              <div className="text-red-400 whitespace-pre-wrap font-mono text-sm">
                {`$ dnsviz probe -a . -o probe.json ${domain}\n$ dnsviz grok ...\n\nERROR:\n${error}\n\nPlease verify the domain name or Backend configuration.`}
              </div>
              <button 
                onClick={() => setError(null)}
                className="mt-4 px-4 py-1 border border-red-500 text-red-400 hover:bg-red-500 hover:text-white rounded text-xs"
              >
                Dismiss
              </button>
            </div>
          </div>
        )}

        {/* Graph Area */}
        <div className="flex-1 h-full relative">
          {graphData && !error ? (
            <DnssecGraph
              nodes={graphData.nodes}
              edges={graphData.edges}
              onNodeClick={setSelectedNode}
            />
          ) : (
            <div className="flex items-center justify-center h-full text-gray-600">
              {loading ? (
                <div className="animate-pulse flex flex-col items-center">
                  <div className="text-emerald-500 text-4xl mb-4">⟳</div>
                  <span>Running DNSViz... This may take a minute.</span>
                </div>
              ) : (
                <span>Enter a domain to visualize its DNSSEC chain</span>
              )}
            </div>
          )}
        </div>

        {/* Side Panel for Metadata */}
        {selectedNode && (
          <aside className="w-80 border-l border-gray-800 bg-gray-900 flex flex-col h-full shadow-[-4px_0_15px_rgba(0,0,0,0.5)] z-10 transition-transform">
            <div className="p-4 border-b border-gray-800 flex justify-between items-center bg-black">
              <h2 className="font-bold text-emerald-400">Node Metadata</h2>
              <button onClick={() => setSelectedNode(null)} className="text-gray-500 hover:text-white">✕</button>
            </div>
            <div className="p-4 flex-1 overflow-auto bg-gray-950">
              <pre className="text-xs text-green-300 whitespace-pre-wrap break-all">
                {JSON.stringify(selectedNode, null, 2)}
              </pre>
            </div>
          </aside>
        )}
      </main>
    </div>
  );
}

export default function DnssecPage() {
  return (
    <Suspense fallback={
      <div className="flex items-center justify-center h-screen bg-gray-950 text-emerald-500 font-mono">
        Loading DNSSEC Visualizer...
      </div>
    }>
      <DnssecContent />
    </Suspense>
  );
}
