import Link from "next/link";

import { auth } from "@clerk/nextjs/server";

import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import WaitlistButton from "./WaitlistButton";
import { prisma } from "@/lib/prisma";

type WaitlistRecord = { userId: string; orgId: string } | null;

export default async function BillingPage() {
  const { userId } = await auth();

  if (!userId) {
    return (
      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>Sign-in Required</CardTitle>
            <CardDescription>Please sign in to view billing options.</CardDescription>
          </CardHeader>
          <CardFooter>
            <Button asChild>
              <Link href="/sign-in">Sign In</Link>
            </Button>
          </CardFooter>
        </Card>
      </div>
    );
  }

  const orgMember = await prisma.orgMember.findFirst({ where: { userId } });

  const orgId = orgMember?.orgId ?? null;

  const isOnWaitlist: WaitlistRecord = orgId
    ? await prisma.waitlist.findFirst({ where: { userId, orgId } })
    : null;

  return (
    <div className="grid gap-6 md:grid-cols-2">
      <Card className="h-full">
        <CardHeader>
          <CardTitle>Free</CardTitle>
          <CardDescription>Perfect for personal projects and small teams.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <span className="text-3xl font-semibold">$0</span>
          <p className="text-sm text-muted-foreground">2GB of cache storage included</p>
          <ul className="mt-4 space-y-2 text-sm text-muted-foreground">
            <li>Up to 2 concurrent tasks</li>
            <li>Community support</li>
            <li>Velocity CLI integration</li>
          </ul>
        </CardContent>
        <CardFooter>
          <Button disabled variant="secondary">
            Current Plan
          </Button>
        </CardFooter>
      </Card>

      <Card className="h-full border-primary">
        <CardHeader>
          <CardTitle>Pro</CardTitle>
          <CardDescription>Scale your team with higher limits and priority support.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <span className="text-3xl font-semibold">$99</span>
          <p className="text-sm text-muted-foreground">50GB of cache storage + team features</p>
          <ul className="mt-4 space-y-2 text-sm text-muted-foreground">
            <li>Unlimited concurrent tasks</li>
            <li>Priority support with response SLAs</li>
            <li>Advanced analytics and usage insights</li>
            <li>SLA-backed reliability</li>
          </ul>
        </CardContent>
        <CardFooter>
          <WaitlistButton isAlreadyOnWaitlist={!!isOnWaitlist} />
        </CardFooter>
      </Card>
    </div>
  );
}
