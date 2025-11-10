'use client';

import { useState } from "react";

import { Button } from "@/components/ui/button";

interface CopyProjectIdButtonProps {
  value: string;
}

export default function CopyProjectIdButton({ value }: CopyProjectIdButtonProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch (error) {
      console.error("Failed to copy project id", error);
    }
  };

  return (
    <Button
      type="button"
      onClick={handleCopy}
      className={copied ? "bg-zinc-200 text-zinc-900 hover:bg-zinc-200" : undefined}
    >
      {copied ? "Copied" : "Copy"}
    </Button>
  );
}
