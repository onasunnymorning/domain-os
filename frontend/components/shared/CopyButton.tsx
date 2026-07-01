"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Copy, Check } from "lucide-react";

interface CopyButtonProps {
  text: string;
  className?: string;
}

export function CopyButton({ text, className }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async (e: React.MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error("Failed to copy text:", err);
    }
  };

  return (
    <Button
      variant="ghost"
      size="icon"
      className={`h-7 w-7 text-muted-foreground hover:text-foreground hover:bg-muted cursor-pointer transition-all duration-200 ${className}`}
      onClick={handleCopy}
      title="Copy to clipboard"
      type="button"
    >
      {copied ? (
        <Check className="h-3.5 w-3.5 text-emerald-500 scale-110 transition-transform" />
      ) : (
        <Copy className="h-3.5 w-3.5 transition-transform group-hover:scale-105" />
      )}
    </Button>
  );
}
