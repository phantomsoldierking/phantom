import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Phantom | Keyboard-First Developer Ops",
  description:
    "Phantom is a terminal dashboard for logs, processes, ports, HTTP workflows, explorer tooling, and project launchers.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
