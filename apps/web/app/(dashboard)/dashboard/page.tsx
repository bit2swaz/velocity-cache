import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function DashboardPage() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Welcome to your Dashboard</CardTitle>
      </CardHeader>
      <CardContent>
        Select a page from the sidebar to get started.
      </CardContent>
    </Card>
  );
}
