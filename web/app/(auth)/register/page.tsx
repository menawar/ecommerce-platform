import { AuthForm } from "../auth-form";
import { registerAction } from "../actions";

export default function RegisterPage() {
  return (
    <main className="mx-auto max-w-sm px-6 py-16">
      <h1 className="text-2xl font-semibold">Create your account</h1>
      <AuthForm action={registerAction} mode="register" />
    </main>
  );
}
