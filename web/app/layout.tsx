import type { Metadata } from "next";
import { Hanken_Grotesk } from "next/font/google";
import "./globals.css";
import { Nav } from "./nav";
import { Footer } from "./footer";
import { CookieNotice } from "./cookie-notice";

const hanken = Hanken_Grotesk({
  variable: "--font-hanken",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700", "800"],
});

export const metadata: Metadata = {
  title: "Plateau — Fresh from the Jos Plateau",
  description:
    "Raw food materials, fresh from the Jos Plateau. Connecting Plateau farms and co-ops directly to your kitchen.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={hanken.variable}>
      <body>
        <Nav />
        {children}
        <Footer />
        <CookieNotice />
      </body>
    </html>
  );
}
