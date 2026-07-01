import type { ElementType, HTMLAttributes } from "react";
import { cn } from "@/lib/cn";

// Container centers content at a max width with responsive horizontal padding —
// replacing the repeated inline `maxWidth / margin:auto / padding` on page <main>s.
// `as` lets a page render it as the <main> landmark; `size` picks the max width.
const sizeMap = {
  sm: "max-w-[480px]", // narrow forms (auth)
  md: "max-w-[640px]", // account, single-column pages
  lg: "max-w-[1080px]",
  xl: "max-w-[1180px]", // full storefront width
} as const;

export function Container({
  as: Comp = "div",
  size = "xl",
  className,
  ...props
}: HTMLAttributes<HTMLElement> & { as?: ElementType; size?: keyof typeof sizeMap }) {
  return <Comp className={cn("mx-auto w-full px-4 sm:px-6", sizeMap[size], className)} {...props} />;
}
