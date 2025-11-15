"use client";

import { FormEvent, useState, useTransition } from "react";

import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

import { createProject } from "./actions";

export default function CreateProjectForm() {
  const [isPending, startTransition] = useTransition();
  const [formError, setFormError] = useState<string | null>(null);
  const [projectName, setProjectName] = useState("");

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = projectName.trim();
    if (!trimmed) {
      setFormError("Project name is required.");
      return;
    }

    const formData = new FormData(event.currentTarget);
    formData.set("name", trimmed);

    startTransition(async () => {
      const result = await createProject(formData);
      if (result.success) {
        setProjectName("");
        setFormError(null);
      } else {
        setFormError(result.error ?? "Something went wrong. Please try again.");
      }
    });
  };

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-3 sm:flex-row sm:items-center">
      <Input
        name="name"
        placeholder="Project Name"
        value={projectName}
        onChange={(event) => setProjectName(event.target.value)}
        disabled={isPending}
        aria-label="Project name"
        className="sm:max-w-xs"
      />
      <Button type="submit" disabled={isPending}>
        {isPending ? (
          <span className="flex items-center gap-2">
            <Loader2 className="h-4 w-4 animate-spin" />
            Creating...
          </span>
        ) : (
          "Create Project"
        )}
      </Button>
      {formError && <p className="text-sm text-muted-foreground sm:ml-3">{formError}</p>}
    </form>
  );
}
