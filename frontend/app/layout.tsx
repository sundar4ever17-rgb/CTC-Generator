import type { Metadata } from "next";
import "./globals.css";
export const metadata: Metadata = { title: "CTC Studio · Compensation workspace", description: "A considered workspace for accurate compensation structures, configuration, and offers." };
export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
