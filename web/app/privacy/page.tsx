import type { Metadata } from "next";
import Link from "next/link";
import { LegalPage, Section } from "../legal-page";

export const metadata: Metadata = {
  title: "Privacy Policy — Plateau",
  description: "How Plateau collects, uses, and protects your personal data, and your data rights.",
};

export default function PrivacyPage() {
  return (
    <LegalPage title="Privacy Policy" lastUpdated="1 July 2026">
      <p style={{ marginBottom: 20 }}>
        This policy explains what personal data Plateau collects, how we use it, and the rights you
        have over it. We handle your data in line with the Nigeria Data Protection Regulation (NDPR)
        and equivalent principles under the GDPR.
      </p>

      <Section heading="What we collect">
        <p>
          Account details (name, email, phone), delivery addresses, your orders and their status,
          and notification history. We do <b>not</b> store card details — payments are handled by our
          provider (Paystack).
        </p>
      </Section>

      <Section heading="How we use it">
        <p>
          To create and secure your account, process and deliver your orders, send transactional
          emails (order updates, verification, password reset), and meet legal obligations. We do not
          sell your personal data.
        </p>
      </Section>

      <Section heading="Cookies">
        <p>
          We use a single <b>strictly necessary</b> cookie to keep you signed in (an httpOnly session
          cookie). It is required for the site to function, so it is not used for tracking or
          advertising. We do not use third-party advertising cookies.
        </p>
      </Section>

      <Section heading="Your data rights">
        <p>
          To exercise any of these rights, contact our data team at{" "}
          <a href="mailto:privacy@plateau.example" style={{ color: "var(--plt-green-text)" }}>
            privacy@plateau.example
          </a>
          . You can also update your profile and addresses directly from your{" "}
          <Link href="/account" style={{ color: "var(--plt-green-text)" }}>
            account page
          </Link>
          .
        </p>
        <ul style={{ margin: "8px 0 0", paddingLeft: 22 }}>
          <li>
            <b>Access / portability</b> — download a machine-readable copy of your data (profile,
            addresses, orders) yourself from your{" "}
            <Link href="/account" style={{ color: "var(--plt-green-text)" }}>
              account page
            </Link>
            .
          </li>
          <li>
            <b>Erasure</b> — delete your account yourself from your{" "}
            <Link href="/account" style={{ color: "var(--plt-green-text)" }}>
              account page
            </Link>
            . We anonymise your personal data across our services; records we must keep for
            legal/accounting reasons (e.g. order totals) are retained without identifying you.
          </li>
          <li>
            <b>Rectification</b> — update your profile and addresses at any time.
          </li>
        </ul>
      </Section>

      <Section heading="Retention">
        <p>
          We keep your data while your account is active. On deletion, identifying data is removed or
          anonymised; anonymised order records may be retained for accounting and fraud-prevention.
        </p>
      </Section>

      <Section heading="Contact">
        <p>
          For any privacy question or to raise a concern, contact our data team at{" "}
          <a href="mailto:privacy@plateau.example" style={{ color: "var(--plt-green-text)" }}>
            privacy@plateau.example
          </a>
          .
        </p>
      </Section>
    </LegalPage>
  );
}
