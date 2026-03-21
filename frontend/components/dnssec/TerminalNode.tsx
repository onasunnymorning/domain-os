import React from 'react';
import { Handle, Position } from '@xyflow/react';

interface TerminalNodeProps {
  data: {
    label: string;
    status: string;
    raw: any;
  };
}

export function TerminalNode({ data }: TerminalNodeProps) {
  // Determine border and text colors based on status
  let borderColor = 'border-gray-500';
  let textColor = 'text-gray-300';
  
  const status = data.status.toLowerCase();
  if (status.includes('secure') || status.includes('valid')) {
    borderColor = 'border-emerald-500';
    textColor = 'text-emerald-400';
  } else if (status.includes('bogus') || status.includes('invalid') || status.includes('error')) {
    borderColor = 'border-red-500';
    textColor = 'text-red-400';
  } else if (status.includes('insecure') || status.includes('warning')) {
    borderColor = 'border-yellow-500';
    textColor = 'text-yellow-400';
  }

  const raw = data.raw || {};
  const isCryptoNode = data.label.includes('[DS]') || data.label.includes('[KSK]') || data.label.includes('[ZSK]');
  
  // Extract algorithm string if available in the description (e.g. "algorithm 8 (RSA/SHA-256)")
  let algoStr = '';
  if (raw.description) {
    const match = raw.description.match(/algorithm \d+ \((.*?)\)/);
    if (match) {
      algoStr = match[1];
    } else if (raw.algorithm) {
      algoStr = `Type ${raw.algorithm}`;
    }
  }

  return (
    <div className={`px-4 py-2 font-mono text-xs bg-gray-900 border-2 ${borderColor} rounded shadow-lg min-w-[160px]`}>
      <Handle type="target" position={Position.Top} className="!bg-gray-500" />
      
      <div className="flex flex-col">
        <span className={`font-bold ${textColor}`}>{data.label}</span>
        
        {isCryptoNode && (
          <div className="mt-1.5 mb-1.5 text-[10px] text-gray-400 space-y-0.5 border-t border-b border-gray-700/50 py-1.5">
             {algoStr && <div>Algo: <span className="text-gray-300">{algoStr}</span></div>}
             {raw.key_length && <div>Bits: <span className="text-gray-300">{raw.key_length}</span></div>}
             {raw.digest_type !== undefined && <div>Digest: <span className="text-gray-300">Type {raw.digest_type}</span></div>}
          </div>
        )}
        
        <span className="text-gray-500 mt-1 uppercase text-[10px] font-bold">
          <span className={textColor === 'text-gray-300' ? 'text-gray-500' : textColor}>
            {data.status}
          </span>
        </span>
      </div>
      
      <Handle type="source" position={Position.Bottom} className="!bg-gray-500" />
    </div>
  );
}
