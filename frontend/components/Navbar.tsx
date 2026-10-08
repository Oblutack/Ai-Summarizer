"use client";
import Link from "next/link";
import { useAuth } from "../contexts/AuthContext";
import { useRouter } from "next/navigation";
import LanguageSwitcher from "./LanguageSwitcher";
import { useT } from "./I18nProvider";
import ThemeToggle from "./ThemeToggle";

export default function Navbar() {
  const { user, logout, loading } = useAuth();
  const router = useRouter();
  const t = useT();

  const handleLogout = async () => {
    await logout();
    router.push("/login");
  };

  if (loading) return null;

  return (
    <header className="border-b-2 border-ink py-4 px-4 sm:px-8">
      <nav className="container mx-auto flex flex-col sm:flex-row justify-between items-center text-2xl uppercase tracking-widest space-y-4 sm:space-y-0">
        <Link href={user ? "/dashboard" : "/"} className="font-bold">
          Inkling
        </Link>
        <div className="space-x-8">
          {user ? (
            <>
              <Link href="/dashboard" className="hover:opacity-70">
                {t("nav.dashboard")}
              </Link>
              <Link href="/account" className="hover:opacity-70">
                {t("nav.account")}
              </Link>
              <button onClick={handleLogout} className="hover:opacity-70">
                {t("nav.logout")}
              </button>
              <ThemeToggle />
              <LanguageSwitcher />
            </>
          ) : (
            <>
              <Link href="/login" className="hover:opacity-70">
                {t("nav.login")}
              </Link>
              <Link href="/signup" className="hover:opacity-70">
                {t("nav.signup")}
              </Link>
              <ThemeToggle />
              <LanguageSwitcher />
            </>
          )}
        </div>
      </nav>
    </header>
  );
}
