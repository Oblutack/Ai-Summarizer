import Link from "next/link";

export default function Navbar() {
  return (
    <nav className="bg-gray-800 text-white p-4">
      <div className="container mx-auto flex justify-between items-center">
        <Link href="/" className="font-bold text-xl">
          AI Summarizer
        </Link>
        <div className="space-x-4">
          <Link href="/login" className="hover:text-gray-300">
            Login
          </Link>
          <Link href="/signup" className="hover:text-gray-300">
            Sign Up
          </Link>
        </div>
      </div>
    </nav>
  );
}
