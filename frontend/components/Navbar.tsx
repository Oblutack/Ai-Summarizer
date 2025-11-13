"use client";
import Link from "next/link";
import { useAuth } from "../contexts/AuthContext";
import { useRouter } from "next/navigation";

export default function Navbar() {
  const { user, logout, loading } = useAuth();
  const router = useRouter();

  const handleLogout = () => {
    logout();
    router.push("/login");
  };

  if (loading) return null;

  return (
    <header className="border-b-2 border-ink py-4 px-4 sm:px-8">
      <nav className="container mx-auto flex flex-col sm:flex-row justify-between items-center text-2xl uppercase tracking-widest space-y-4 sm:space-y-0">
        <Link href={user ? "/dashboard" : "/"} className="font-bold">
          AI Summarizer
        </Link>
        <div className="space-x-8">
          {user ? (
            <>
              <Link href="/dashboard" className="hover:opacity-70">
                Dashboard
              </Link>
              <button onClick={handleLogout} className="hover:opacity-70">
                Logout
              </button>
            </>
          ) : (
            <>
              <Link href="/login" className="hover:opacity-70">
                Login
              </Link>
              <Link href="/signup" className="hover:opacity-70">
                Sign Up
              </Link>
            </>
          )}
        </div>
      </nav>
    </header>
  );
}
