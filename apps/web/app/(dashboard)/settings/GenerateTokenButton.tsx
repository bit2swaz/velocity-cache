'use client';

import { useTransition } from "react";

import { Button } from "@/components/ui/button";

function noopGenerateToken() {
  // TODO: Wire this button to the real token generation endpoint.
  return Promise.resolve();
}

export default function GenerateTokenButton() {
  const [isPending, startTransition] = useTransition();

  const handleClick = () => {
    startTransition(async () => {
      try {
        await noopGenerateToken();
      } catch (error) {
        console.error("Failed to generate token", error);
      }
    });
  };

  return (
    <Button onClick={handleClick} disabled={isPending} className="mt-4">
      {isPending ? "Generating..." : "Generate API Token"}
    </Button>
  );
}
