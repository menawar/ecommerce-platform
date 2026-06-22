import { AuthForm } from "../auth-form";
import { loginAction } from "../actions";

// Server Component: it imports the Server Action and hands it to the client form.
// Passing a server action as a prop is allowed — it's a serializable reference, not
// the function body, so no server code ships to the browser.
export default function LoginPage() {
  return (
    <main className="mx-auto max-w-sm px-6 py-16">
      <h1 className="text-2xl font-semibold">Sign in</h1>
      <AuthForm action={loginAction} mode="login" />
    </main>
  );
}
