import type { Metadata } from "next";
import "./globals.css";
// Per-request rendering is required for the CSP nonce and sensitive workspace.
export const dynamic = "force-dynamic";
export const metadata: Metadata = { title: "CTC Studio · Compensation workspace", description: "A considered workspace for accurate compensation structures, configuration, and offers." };
export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
