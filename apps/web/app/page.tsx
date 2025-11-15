import Link from "next/link";

import { SignInButton, UserButton } from "@clerk/nextjs";
import { auth } from "@clerk/nextjs/server";

import { Button } from "@/components/ui/button";

export default async function Home() {
  const { userId } = await auth();

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-6 bg-gradient-to-br from-zinc-950 via-zinc-900 to-black px-6 py-16 text-center text-zinc-100">
      <h1 className="text-4xl font-bold tracking-tight sm:text-6xl">VelocityCache</h1>
      <p className="max-w-xl text-lg text-zinc-400 sm:text-xl">
        Turbocharge your build pipelines with smart caching for teams.
      </p>

      <div className="flex flex-col items-center gap-4 sm:flex-row">
        {userId ? (
          <>
            <UserButton afterSignOutUrl="/" />
            <Button asChild size="lg" className="rounded-full">
              <Link href="/dashboard">Go to Dashboard</Link>
            </Button>
          </>
        ) : (
          <SignInButton mode="modal">
            <Button size="lg" className="rounded-full">
              Sign in to VelocityCache
            </Button>
          </SignInButton>
        )}
      </div>
    </main>
  );
}
