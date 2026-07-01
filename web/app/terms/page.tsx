import type { Metadata } from "next";
import Link from "next/link";
import { LegalPage, Section } from "../legal-page";

export const metadata: Metadata = {
  title: "Terms of Service — Plateau",
  description: "The terms governing your use of the Plateau storefront.",
};

export default function TermsPage() {
  return (
    <LegalPage title="Terms of Service" lastUpdated="1 July 2026">
      <p style={{ marginBottom: 20 }}>
        These terms govern your use of Plateau (&quot;we&quot;, &quot;us&quot;), an online marketplace
        for raw food materials sourced from farms and co-ops across the Jos Plateau. By creating an
        account or placing an order you agree to these terms.
      </p>

      <Section heading="1. Orders and pricing">
        <p>
          All prices are shown in Nigerian Naira (₦) and include applicable charges shown at
          checkout. Placing an order is an offer to buy; the order is confirmed once payment is
          received and stock is reserved. We may cancel and fully refund an order if an item is
          unavailable or a pricing error is detected.
        </p>
      </Section>

      <Section heading="2. Payment">
        <p>
          Payments are processed by our third-party payment provider (Paystack). We do not store
          your card details on our servers. You confirm you are authorised to use the payment method
          you provide.
        </p>
      </Section>

      <Section heading="3. Delivery and refunds">
        <p>
          We deliver across Jos and Plateau State. Delivery times are estimates. If an order is
          cancelled before dispatch, or a refund is issued, funds are returned to your original
          payment method. Perishable goods may be non-returnable once delivered except where they
          arrive damaged or not as described.
        </p>
      </Section>

      <Section heading="4. Your account">
        <p>
          You are responsible for keeping your account credentials secure and for activity under
          your account. Notify us promptly of any unauthorised use. You may close your account and
          request deletion of your data at any time — see our{" "}
          <Link href="/privacy" style={{ color: "var(--plt-green-text)" }}>
            Privacy Policy
          </Link>
          .
        </p>
      </Section>

      <Section heading="5. Acceptable use">
        <p>
          You agree not to misuse the service — including attempting to disrupt it, access it without
          authorisation, or use it for unlawful purposes.
        </p>
      </Section>

      <Section heading="6. Liability">
        <p>
          The service is provided on a reasonable-efforts basis. To the extent permitted by law, our
          liability for any claim relating to an order is limited to the amount you paid for that
          order.
        </p>
      </Section>

      <Section heading="7. Changes and contact">
        <p>
          We may update these terms; material changes will be posted here with a new date. Questions?
          Contact us at{" "}
          <a href="mailto:support@plateau.example" style={{ color: "var(--plt-green-text)" }}>
            support@plateau.example
          </a>
          .
        </p>
      </Section>
    </LegalPage>
  );
}
