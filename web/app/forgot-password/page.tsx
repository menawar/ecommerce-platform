import { ForgotForm } from "./forgot-form";

export default function ForgotPasswordPage() {
  return (
    <main style={{ maxWidth: 420, margin: "0 auto", padding: "60px 20px" }}>
      <div className="plt-card-lg" style={{ borderRadius: "var(--plt-radius-xl)", padding: "36px 32px" }}>
        <h1 style={{ fontSize: 22, fontWeight: 800, textAlign: "center", margin: 0 }}>Reset your password</h1>
        <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", textAlign: "center", margin: "6px 0 24px" }}>
          Enter your email and we’ll send you a reset link.
        </p>
        <ForgotForm />
      </div>
    </main>
  );
}
