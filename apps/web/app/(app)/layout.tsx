import Link from "next/link";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      <header className="mb-8 flex items-center justify-between">
        <Link href="/" className="text-2xl font-bold tracking-tight">
          Sorolens
        </Link>
      </header>
      <main>{children}</main>
    </div>
  );
}
