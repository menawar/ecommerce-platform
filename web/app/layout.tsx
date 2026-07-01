import type { Metadata } from "next";
import { Hanken_Grotesk } from "next/font/google";
import "./globals.css";
import { Nav } from "./nav";
import { Footer } from "./footer";
import { CookieNotice } from "./cookie-notice";
import { SITE_URL } from "@/lib/site";

const hanken = Hanken_Grotesk({
  variable: "--font-hanken",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700", "800"],
});

const DESCRIPTION =
  "Raw food materials, fresh from the Jos Plateau. Connecting Plateau farms and co-ops directly to your kitchen.";

export const metadata: Metadata = {
  // metadataBase lets Next resolve relative OG/canonical URLs to absolute ones.
  metadataBase: new URL(SITE_URL),
  // Per-page titles render as "Shop — Plateau"; the home page uses the default.
  title: {
    default: "Plateau — Fresh from the Jos Plateau",
    template: "%s — Plateau",
  },
  description: DESCRIPTION,
  applicationName: "Plateau",
  openGraph: {
    type: "website",
    siteName: "Plateau",
    title: "Plateau — Fresh from the Jos Plateau",
    description: DESCRIPTION,
    locale: "en_NG",
  },
  twitter: { card: "summary_large_image", title: "Plateau", description: DESCRIPTION },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={hanken.variable}>
      <body>
        {/* Keyboard/screen-reader users can jump past the nav straight to content. */}
        <a href="#main-content" className="plt-skip-link">
          Skip to content
        </a>
        <Nav />
        {/* tabIndex=-1 so the skip link moves keyboard focus here (a plain div isn't
            focusable), letting the next Tab continue into the content, not the nav. */}
        <div id="main-content" tabIndex={-1} style={{ outline: "none" }}>
          {children}
        </div>
        <Footer />
        <CookieNotice />
      </body>
    </html>
  );
}
