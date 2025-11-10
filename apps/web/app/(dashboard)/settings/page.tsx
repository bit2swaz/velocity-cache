import Link from "next/link";
import { redirect } from "next/navigation";

import { auth } from "@clerk/nextjs/server";

import CopyProjectIdButton from "./CopyProjectIdButton";
import GenerateTokenButton from "./GenerateTokenButton";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { prisma } from "@/lib/prisma";

export default async function SettingsPage() {
  const { userId } = await auth();

  if (!userId) {
    redirect("/sign-in");
  }

  const orgMember = await prisma.orgMember.findFirst({
    where: { userId },
    include: { organization: true },
  });

  const [projects, apiTokens] = await Promise.all([
    orgMember
      ? prisma.project.findMany({ where: { orgId: orgMember.orgId } })
      : Promise.resolve([]),
    prisma.apiToken.findMany({ where: { userId }, orderBy: { createdAt: "desc" } }),
  ]);

  if (!orgMember) {
    return (
      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>Organization Required</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <p className="text-sm text-zinc-600 dark:text-zinc-400">
              You are not a member of any organization yet. Join or create an
              organization to manage projects and API tokens.
            </p>
            <Button asChild>
              <Link href="/create-organization">Create Organization</Link>
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <Card>
        <CardHeader>
          <CardTitle>{orgMember.organization?.name ?? "Projects"}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {projects.length === 0 ? (
            <div className="flex flex-col gap-3 rounded-lg border border-dashed border-zinc-200 p-6 text-sm text-zinc-500 dark:border-zinc-800 dark:text-zinc-400 md:flex-row md:items-center md:justify-between">
              <span>No projects found for this organization.</span>
              <Button asChild variant="secondary">
                <Link href="/projects/new">Create Project</Link>
              </Button>
            </div>
          ) : (
            <div className="space-y-4">
              {projects.map((project) => (
                <div
                  key={project.id}
                  className="space-y-3 rounded-lg border border-zinc-200 p-4 dark:border-zinc-800"
                >
                  <div className="flex flex-col justify-between gap-1 md:flex-row md:items-center">
                    <p className="text-sm font-medium text-zinc-900 dark:text-zinc-100">
                      {project.name}
                    </p>
                    <span className="text-xs uppercase tracking-wide text-zinc-500 dark:text-zinc-400">
                      Project ID
                    </span>
                  </div>
                  <div className="flex flex-col gap-2 md:flex-row md:items-center">
                    <Input readOnly value={project.id} />
                    <CopyProjectIdButton value={project.id} />
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>API Tokens</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {apiTokens.length === 0 ? (
            <p className="text-sm text-zinc-500 dark:text-zinc-400">
              You have not generated any API tokens yet.
            </p>
          ) : (
            <div className="space-y-3">
              {apiTokens.map((token) => (
                <div
                  key={token.id}
                  className="rounded-lg border border-zinc-200 p-4 dark:border-zinc-800"
                >
                  <p className="text-sm font-medium text-zinc-900 dark:text-zinc-100">
                    {token.note || "Untitled token"}
                  </p>
                  <p className="text-xs text-zinc-500 dark:text-zinc-400">
                    Created {token.createdAt.toLocaleString()}
                  </p>
                </div>
              ))}
            </div>
          )}

          <GenerateTokenButton />
        </CardContent>
      </Card>
    </div>
  );
}
