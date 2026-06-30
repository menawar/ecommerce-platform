import { AuthForm } from "../auth-form";
import { loginAction } from "../actions";

// Server Component: it imports the Server Action and hands it to the client form.
// Passing a server action as a prop is allowed — it's a serializable reference, not
// the function body, so no server code ships to the browser.
export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ reset?: string }>;
}) {
  const { reset } = await searchParams;
  return (
    <main
      style={{
        maxWidth: 420,
        margin: "0 auto",
        padding: "60px 20px",
      }}
    >
      <div
        className="plt-card-lg"
        style={{
          borderRadius: "var(--plt-radius-xl)",
          padding: "36px 32px",
        }}
      >
        <div style={{ textAlign: "center", marginBottom: 8 }}>
          <svg
            width="36"
            height="36"
            viewBox="0 0 32 32"
            fill="none"
            style={{ margin: "0 auto" }}
          >
            <path
              d="M1 25 L11 10 L17 18.5 L22 11 L31 25 Z"
              fill="#7fb56a"
            />
            <path d="M1 25 L11 10 L15.5 16.4 L8 25 Z" fill="#5f9a4d" />
            <circle cx="24.5" cy="8" r="3.2" fill="#f3b73f" />
          </svg>
        </div>
        <h1
          style={{
            fontSize: 22,
            fontWeight: 800,
            textAlign: "center",
            margin: 0,
          }}
        >
          Sign in
        </h1>
        <p
          style={{
            fontSize: 14,
            color: "var(--plt-text-secondary)",
            textAlign: "center",
            margin: "6px 0 0",
          }}
        >
          Welcome back to Plateau
        </p>
        {reset === "1" && (
          <div
            style={{
              fontSize: 13,
              color: "var(--plt-green-deep)",
              background: "var(--plt-success-bg, #e8f3e3)",
              padding: "10px 12px",
              borderRadius: "var(--plt-radius-sm)",
              marginTop: 16,
              textAlign: "center",
            }}
          >
            Password updated — sign in with your new password.
          </div>
        )}
        <AuthForm action={loginAction} mode="login" />
      </div>
    </main>
  );
}
