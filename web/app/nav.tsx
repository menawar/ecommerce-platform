import Link from "next/link";

import { isLoggedIn } from "@/lib/session";
import { logoutAction } from "@/app/(auth)/actions";

// An async Server Component: it reads the session cookie on the server to decide
// which links to show. Reading cookies makes the layout render per-request
// (dynamic) — exactly what an auth-aware header needs.
export async function Nav() {
  const loggedIn = await isLoggedIn();

  return (
    <header className="border-b border-zinc-200">
      <nav className="mx-auto flex max-w-5xl items-center justify-between px-6 py-3">
        <Link href="/" className="font-semibold">
          E-Commerce
        </Link>
        <div className="flex items-center gap-4 text-sm">
          <Link href="/products" className="hover:underline">
            Products
          </Link>
          {loggedIn ? (
            <>
              <Link href="/account" className="hover:underline">
                Account
              </Link>
              <form action={logoutAction}>
                <button className="text-zinc-600 hover:underline">Log out</button>
              </form>
            </>
          ) : (
            <>
              <Link href="/login" className="hover:underline">
                Sign in
              </Link>
              <Link href="/register" className="hover:underline">
                Register
              </Link>
            </>
          )}
        </div>
      </nav>
    </header>
  );
}
