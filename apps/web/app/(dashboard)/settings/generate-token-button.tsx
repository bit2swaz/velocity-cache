'use client';

import React, { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

import { generateApiToken } from "@/src/app/(dashboard)/settings/actions";

export default function GenerateTokenButton() {
  const [note, setNote] = useState("");
  const [generatedToken, setGeneratedToken] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleGenerate = async () => {
    if (isSubmitting) {
      return;
    }

    setIsSubmitting(true);
    try {
      const token = await generateApiToken(note.trim());
      setGeneratedToken(token);
      setNote("");
    } catch (error) {
      console.error("Failed to generate API token", error);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleClose = () => {
    setGeneratedToken(null);
  };

  return (
    <Dialog open={Boolean(generatedToken)} onOpenChange={(open) => !open && handleClose()}>
      <div className="space-y-2">
        <Label htmlFor="token-note">Token note (optional)</Label>
        <Input
          id="token-note"
          placeholder="e.g. CI pipeline"
          value={note}
          onChange={(event) => setNote(event.target.value)}
          disabled={isSubmitting}
        />
      </div>

      <Button onClick={handleGenerate} disabled={isSubmitting} className="mt-4">
        {isSubmitting ? "Generating..." : "Generate New Token"}
      </Button>

      <DialogContent showCloseButton onOpenAutoFocus={(event) => event.preventDefault()}>
        <DialogHeader>
          <DialogTitle>API Token Created</DialogTitle>
          <DialogDescription>
            Your new token has been generated. Copy it now; you will not be able to see it again.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          <Label htmlFor="generated-token">Token</Label>
          <Input id="generated-token" readOnly value={generatedToken ?? ""} />
        </div>
        <div className="flex justify-end">
          <Button type="button" onClick={handleClose} variant="secondary">
            Close
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
