import { auth } from "@clerk/nextjs/server";
import { Prisma } from "@prisma/client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { prisma } from "@/lib/prisma";
import AnalyticsChart from "./analytics-chart";

function formatDuration(ms: number): string {
  if (ms <= 0) return "0 ms";
  const seconds = ms / 1000;
  if (seconds < 60) {
    return `${seconds.toFixed(1)}s`;
  }
  const minutes = seconds / 60;
  if (minutes < 60) {
    return `${minutes.toFixed(1)}m`;
  }
  const hours = minutes / 60;
  if (hours < 24) {
    return `${hours.toFixed(1)}h`;
  }
  const days = hours / 24;
  return `${days.toFixed(1)}d`;
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex++;
  }
  return `${value.toFixed(1)} ${units[unitIndex]}`;
}

export default async function AnalyticsPage() {
  const { userId } = await auth();

  if (!userId) {
    return "Not authenticated";
  }

  const orgMember = await prisma.orgMember.findFirst({
    where: { userId },
  });

  if (!orgMember) {
    return "No org found";
  }

  const [hitCount, missData, storageData, chartData] = await Promise.all([
    prisma.cacheEvent.count({
      where: { project: { orgId: orgMember.orgId }, status: "HIT" },
    }),
    prisma.cacheEvent.aggregate({
      _count: { status: true },
      _sum: { duration: true },
      where: { project: { orgId: orgMember.orgId }, status: "MISS" },
    }),
    prisma.cacheEvent.aggregate({
      _sum: { size: true },
      where: { project: { orgId: orgMember.orgId }, status: "MISS" },
    }),
    prisma.$queryRaw<Array<{ date: Date; status: string; count: bigint }>>(
      Prisma.sql`
        SELECT
          DATE_TRUNC('day', "createdAt") as date,
          status,
          COUNT(*) as count
        FROM "CacheEvent"
        WHERE "projectId" IN (
          SELECT id FROM "Project" WHERE "orgId" = ${orgMember.orgId}
        )
        GROUP BY 1, 2
        ORDER BY 1
      `,
    ),
  ]);

  const totalHits = hitCount;
  const totalMisses = missData._count?.status ?? 0;
  const timeSavedInMs = missData._sum?.duration ?? 0;
  const storageUsedInBytes = storageData._sum?.size ?? 0;
  const totalRequests = totalHits + totalMisses;
  const hitRate = totalRequests > 0 ? Math.round((totalHits / totalRequests) * 100) : 0;

  const normalizedChartData = chartData.map((row) => ({
    date: row.date instanceof Date ? row.date.toISOString() : row.date,
    status: row.status,
    count: Number(row.count),
  }));

  return (
    <div className="space-y-8">
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle>Time Saved</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{formatDuration(timeSavedInMs)}</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Hit Rate</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{hitRate}%</p>
            <p className="text-sm text-muted-foreground">{totalHits} / {totalRequests} requests</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Storage Used</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{formatBytes(storageUsedInBytes)}</p>
          </CardContent>
        </Card>
      </div>

      <AnalyticsChart rawData={normalizedChartData} />
    </div>
  );
}
