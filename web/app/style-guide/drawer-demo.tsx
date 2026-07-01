"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Drawer } from "@/components/ui/drawer";

// Interactive demo of the Drawer for the style guide (the style guide itself is a
// Server Component, so the open state lives here).
export function DrawerDemo() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button variant="outline" onClick={() => setOpen(true)}>
        Open drawer
      </Button>
      <Drawer open={open} onClose={() => setOpen(false)} title="Menu" side="left">
        <nav className="flex flex-col gap-2 text-sm">
          <a href="#" className="rounded-md px-2 py-2 hover:bg-surface">
            All produce
          </a>
          <a href="#" className="rounded-md px-2 py-2 hover:bg-surface">
            This week&apos;s harvest
          </a>
          <a href="#" className="rounded-md px-2 py-2 hover:bg-surface">
            Bulk orders
          </a>
        </nav>
      </Drawer>
    </>
  );
}
