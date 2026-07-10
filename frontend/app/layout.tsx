import type { Metadata } from "next";
import { Inter, IBM_Plex_Mono } from "next/font/google";
import { PublicEnvScript } from "next-runtime-env";
import "./globals.css";
import { Providers } from "@/components/providers";
import { Toaster } from "@/components/ui/sonner";

const inter = Inter({ subsets: ["latin"] });
const ibmPlexMono = IBM_Plex_Mono({
  weight: ['300', '400'],
  subsets: ['latin'],
  variable: '--font-console',
});

export const metadata: Metadata = {
  title: 'CO Registry',
  description: "Registry administration dashboard",
  icons: {
    icon: '/favicon-co.svg',
    apple: '/favicon-co.svg',
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        {/* disableNextScript emits a plain inline <script> so window.__ENV is set
            before instrumentation-client.ts initialises PostHog. next/script's
            beforeInteractive strategy does not guarantee that ordering. */}
        <PublicEnvScript disableNextScript />
      </head>
      <body className={`${inter.className} ${ibmPlexMono.variable}`}>
        <Providers>
          {children}
          <Toaster />
        </Providers>
      </body>
    </html>
  );
}
