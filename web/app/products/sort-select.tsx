"use client";

import type { CSSProperties } from "react";

type Option = { value: string; label: string };

// SortSelect is the one sliver of client JS on the products page: a plain <select>
// that submits its enclosing form on change, so picking a sort navigates to
// /products?sort=… (and keeps the hidden q). The page itself stays a Server
// Component — this exists only because a server-rendered <select> can't auto-submit
// without client JS.
export function SortSelect({
  options,
  value,
  style,
}: {
  options: Option[];
  value: string;
  style?: CSSProperties;
}) {
  return (
    <select
      name="sort"
      defaultValue={value}
      onChange={(e) => e.currentTarget.form?.requestSubmit()}
      style={style}
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  );
}
