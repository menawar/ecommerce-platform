import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// cn merges conditional class lists (clsx) and de-conflicts Tailwind utilities
// (tailwind-merge), so a later `px-4` wins over an earlier `px-2`. This is the
// composition primitive every UI component uses to make variants overridable.
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
