'use client';
import React, { useCallback, useMemo } from 'react';
import { ReactFlow, Controls, Background, useNodesState, useEdgesState, BackgroundVariant, Node } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { TerminalNode } from './TerminalNode';

interface DnssecGraphProps {
  nodes: any[];
  edges: any[];
  onNodeClick: (node: any) => void;
}

export function DnssecGraph({ nodes: initialNodes, edges: initialEdges, onNodeClick }: DnssecGraphProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges);

  // Define custom node types
  const nodeTypes = useMemo(() => ({ terminalNode: TerminalNode }), []);

  const handleNodeClick = useCallback((event: React.MouseEvent, node: Node) => {
    onNodeClick(node.data.raw);
  }, [onNodeClick]);

  return (
    <div className="w-full h-full bg-black rounded-lg overflow-hidden border border-gray-800">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={handleNodeClick}
        nodeTypes={nodeTypes}
        fitView
        colorMode="dark"
      >
        <Background variant={BackgroundVariant.Dots} gap={12} size={1} color="#333" />
        <Controls className="!bg-gray-900 !text-white !border-gray-700" />
      </ReactFlow>
    </div>
  );
}
