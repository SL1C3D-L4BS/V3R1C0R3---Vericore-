import type { Metadata } from "next";
import { TelemetryProvider } from "./TelemetryProvider";
import "./globals.css";

export const metadata: Metadata = {
  title: "V3R1C0R3 Verification",
  description: "Article 14 double-verification queue",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>
        <TelemetryProvider>{children}</TelemetryProvider>
      </body>
    </html>
  );
}
