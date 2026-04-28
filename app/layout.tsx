export const metadata = {
  title: 'favicon-fisher',
  description: 'Find the favicon used by any website.',
};

import './globals.css';

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
