import { SignedIn, SignedOut } from "@clerk/nextjs";
import Image from "next/image";

export default function Home() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-50 font-sans dark:bg-black">
      <main className="flex min-h-screen w-full max-w-3xl flex-col items-center justify-between py-32 px-16 bg-white dark:bg-black sm:items-start">
        <Image
          className="dark:invert"
          src="/next.svg"
          alt="Next.js logo"
          width={100}
          height={20}
          priority
        />
        <div className="flex flex-col items-center gap-6 text-center sm:items-start sm:text-left">
          <h1 className="max-w-xs text-3xl font-semibold leading-10 tracking-tight text-black dark:text-zinc-50">
            Welcome to Velocity Cache
          </h1>
          <SignedOut>
            <p className="max-w-md text-lg leading-8 text-zinc-600 dark:text-zinc-400">
              Sign in to unlock workspace dashboards, caches, and deployment
              controls.
            </p>
          </SignedOut>
          <SignedIn>
            <p className="max-w-md text-lg leading-8 text-zinc-600 dark:text-zinc-400">
              You are signed in. Explore your cache configurations and recent
              activity below.
            </p>
          </SignedIn>
        </div>
        <div className="flex flex-col gap-4 text-base font-medium sm:flex-row">
          <a
            className="flex h-12 w-full items-center justify-center gap-2 rounded-full bg-foreground px-5 text-background transition-colors hover:bg-[#383838] dark:hover:bg-[#ccc] md:w-[158px]"
            href="https://clerk.com/docs"
            target="_blank"
            rel="noopener noreferrer"
          >
            <Image
              className="dark:invert"
              src="/vercel.svg"
              alt="Docs icon"
              width={16}
              height={16}
            />
            Clerk Docs
          </a>
          <a
            className="flex h-12 w-full items-center justify-center rounded-full border border-solid border-black/[.08] px-5 transition-colors hover:border-transparent hover:bg-black/[.04] dark:border-white/[.145] dark:hover:bg-[#1a1a1a] md:w-[158px]"
            href="https://nextjs.org/docs/app"
            target="_blank"
            rel="noopener noreferrer"
          >
            App Router Docs
          </a>
        </div>
      </main>
    </div>
  );
}
