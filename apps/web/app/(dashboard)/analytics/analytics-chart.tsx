"use client";

import { useMemo } from "react";
import { Bar, BarChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { ChartContainer, type ChartConfig } from "@/components/ui/chart";

type RawDataRow = {
  date: string;
  status: "HIT" | "MISS" | string;
  count: number | null | undefined;
};

type ProcessedChartRow = {
  date: string;
  HIT: number;
  MISS: number;
};

type AnalyticsChartProps = {
  rawData: RawDataRow[];
};

const chartConfig = {
  HIT: {
    label: "Hits",
    color: "hsl(var(--chart-1))",
  },
  MISS: {
    label: "Misses",
    color: "hsl(var(--chart-2))",
  },
} satisfies ChartConfig;

export default function AnalyticsChart({ rawData }: AnalyticsChartProps) {
  const processedData = useMemo<ProcessedChartRow[]>(() => {
    if (!Array.isArray(rawData) || rawData.length === 0) {
      return [];
    }

    const formatter = new Intl.DateTimeFormat("en-US", {
      month: "short",
      day: "numeric",
      timeZone: "UTC",
    });

    const grouped = new Map<string, ProcessedChartRow>();

    for (const entry of rawData) {
      if (!entry?.date) {
        continue;
      }

      const parsedDate = new Date(entry.date);
      if (Number.isNaN(parsedDate.getTime())) {
        continue;
      }

      const key = formatter.format(parsedDate);
      const row =
        grouped.get(key) ??
        {
          date: key,
          HIT: 0,
          MISS: 0,
        };

      const value = typeof entry.count === "number" && entry.count > 0 ? entry.count : 0;
      if (entry.status === "HIT") {
        row.HIT += value;
      } else if (entry.status === "MISS") {
        row.MISS += value;
      }

      grouped.set(key, row);
    }

    return Array.from(grouped.values());
  }, [rawData]);

  return (
    <ChartContainer config={chartConfig} className="h-[360px] w-full">
      <ResponsiveContainer>
        <BarChart data={processedData}>
          <XAxis dataKey="date" stroke="hsl(var(--muted-foreground))" tickLine={false} axisLine={false} />
          <YAxis stroke="hsl(var(--muted-foreground))" allowDecimals={false} />
          <Tooltip />
          <Bar dataKey="HIT" fill="var(--color-HIT)" stackId="a" radius={[4, 4, 0, 0]} />
          <Bar dataKey="MISS" fill="var(--color-MISS)" stackId="a" radius={[4, 4, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </ChartContainer>
  );
}
