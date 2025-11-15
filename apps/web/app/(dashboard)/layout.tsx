import Link from "next/link";
import type { ReactNode } from "react";

import { UserButton } from "@clerk/nextjs";
import {
  CreditCard,
  File,
  Home,
  LineChart,
  PanelLeft,
  Settings,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";

type DashboardLayoutProps = {
  children: ReactNode;
};

const navItems = [
  { href: "/dashboard", label: "Dashboard", icon: Home },
  { href: "/projects", label: "Projects", icon: File },
  { href: "/analytics", label: "Analytics", icon: LineChart },
  { href: "/billing", label: "Billing", icon: CreditCard },
  { href: "/settings", label: "Settings", icon: Settings },
];

export default function DashboardLayout({ children }: DashboardLayoutProps) {
  return (
    <div className="flex min-h-screen flex-col bg-background">
      <Header />
      <div className="flex flex-1">
        <Sidebar />
        <main className="flex-1 px-6 py-8">{children}</main>
      </div>
    </div>
  );
}

function Header() {
  return (
    <header className="border-b bg-background">
      <div className="flex h-16 items-center justify-between px-4">
        <Sheet>
          <SheetTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="md:hidden"
              aria-label="Open navigation menu"
            >
              <PanelLeft className="h-5 w-5" />
            </Button>
          </SheetTrigger>
          <SheetContent side="left" className="w-64">
            <nav className="mt-6 flex flex-col gap-2">
              {navItems.map((item) => (
                <Button key={item.href} variant="ghost" asChild className="justify-start">
                  <Link href={item.href}>
                    <item.icon className="mr-2 h-4 w-4" />
                    {item.label}
                  </Link>
                </Button>
              ))}
            </nav>
          </SheetContent>
        </Sheet>

        <Link href="/dashboard" className="text-lg font-semibold">
          VelocityCache
        </Link>

        <UserButton afterSignOutUrl="/" />
      </div>
    </header>
  );
}

function Sidebar() {
  return (
    <aside className="hidden w-64 border-r bg-muted/20 md:block">
      {/* <div className="flex h-16 items-center px-6 text-lg font-semibold">VelocityCache</div> */}
      <nav className="flex flex-col gap-1 px-2 pb-6">
        {navItems.map((item) => (
          <Button key={item.href} variant="ghost" asChild className="justify-start">
            <Link href={item.href}>
              <item.icon className="mr-2 h-4 w-4" />
              {item.label}
            </Link>
          </Button>
        ))}
      </nav>
    </aside>
  );
}
