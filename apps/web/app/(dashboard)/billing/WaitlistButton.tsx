"use client";

import { useState } from "react";

import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";

import { joinWaitlist } from "./actions";

type WaitlistButtonProps = {
  isAlreadyOnWaitlist: boolean;
};

export default function WaitlistButton({ isAlreadyOnWaitlist }: WaitlistButtonProps) {
  const [isLoading, setIsLoading] = useState(false);
  const [hasJoined, setHasJoined] = useState(isAlreadyOnWaitlist);
  const [message, setMessage] = useState<string | null>(
    isAlreadyOnWaitlist ? "You're on the list!" : null,
  );

  const handleClick = async () => {
    if (isLoading || hasJoined) {
      return;
    }

    setIsLoading(true);
    try {
      const result = await joinWaitlist();
      if (result?.success) {
        setHasJoined(true);
        setMessage("You're on the list! We'll let you know as soon as Pro is ready.");
      } else {
        setMessage("Something went wrong. Please try again.");
      }
    } catch (error) {
      console.error("Failed to join waitlist", error);
      setMessage("Something went wrong. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="w-full space-y-2">
      <Button
        onClick={handleClick}
        disabled={isAlreadyOnWaitlist || hasJoined || isLoading}
        className="w-full"
        variant={isAlreadyOnWaitlist || hasJoined ? "secondary" : "default"}
      >
        {isLoading ? (
          <span className="flex items-center gap-2">
            <Loader2 className="h-4 w-4 animate-spin" />
            Joining...
          </span>
        ) : hasJoined || isAlreadyOnWaitlist ? (
          "You're on the list!"
        ) : (
          "Join Waitlist"
        )}
      </Button>
      {message && <p className="text-center text-sm text-muted-foreground">{message}</p>}
    </div>
  );
}
