"use client";

import { useState } from "react";
import axios from "axios";
import { useRouter } from "next/navigation";
import FormContainer from "../../components/FormContainer";

export default function SignupPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const router = useRouter();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    try {
      const response = await axios.post(`${process.env.NEXT_PUBLIC_API_URL}/signup`, {
        email: email,
        password: password,
      });

      router.push("/login");
    } catch (err) {
      if (axios.isAxiosError(err) && err.response) {
        setError(err.response.data.error || "An unexpected error occurred.");
      } else {
        setError("An unexpected error occurred.");
      }
    }
  };

  return (
    <FormContainer title="Sign Up">
      <form
        onSubmit={handleSubmit}
        className="w-full flex flex-col items-center space-y-6 text-2xl"
      >
        <div className="w-full">
          <label
            className="block uppercase tracking-wider mb-1"
            htmlFor="email"
          >
            Email
          </label>
          <input
            className="w-full p-3 bg-canvas border-2 border-ink rounded-md focus:outline-none"
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </div>
        <div className="w-full">
          <label
            className="block uppercase tracking-wider mb-1"
            htmlFor="password"
          >
            Password
          </label>
          <input
            className="w-full p-3 bg-canvas border-2 border-ink rounded-md focus:outline-none"
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </div>

        {error && <p className="text-red-500 text-lg">{error}</p>}

        <button
          type="submit"
          className="bg-ink text-canvas text-3xl uppercase font-bold py-3 px-12 rounded-md border-2 border-b-8 border-ink hover:opacity-90 disabled:opacity-50"
        >
          Sign Up
        </button>
      </form>
    </FormContainer>
  );
}
