import { redirect } from "next/navigation";

import { auth } from "@clerk/nextjs/server";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { prisma } from "@/lib/prisma";

import CreateProjectForm from "./create-project-form";

export default async function ProjectsPage() {
  const { userId } = await auth();
  if (!userId) {
    redirect("/sign-in");
  }

  const orgMember = await prisma.orgMember.findFirst({
    where: { userId },
    select: { orgId: true },
  });

  if (!orgMember) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>No Organization Found</CardTitle>
        </CardHeader>
        <CardContent>
          Join or create an organization to manage projects.
        </CardContent>
      </Card>
    );
  }

  const projects = await prisma.project.findMany({
    where: { orgId: orgMember.orgId },
    orderBy: { name: "asc" },
  });

  return (
    <div className="space-y-8">
      <Card>
        <CardHeader>
          <CardTitle>Your Projects</CardTitle>
        </CardHeader>
        <CardContent>
          {projects.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No projects yet. Create one below to get started.
            </p>
          ) : (
            <ul className="divide-y divide-border">
              {projects.map((project) => (
                <li key={project.id} className="py-3">
                  <p className="font-medium text-foreground">{project.name}</p>
                  <p className="text-xs text-muted-foreground">ID: {project.id}</p>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      <CreateProjectForm />
    </div>
  );
}
