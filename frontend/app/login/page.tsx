"use client";

import { useState, useRef, useEffect } from "react";
import axios from "axios";
import { useRouter } from "next/navigation";
import { useAuth } from "../../contexts/AuthContext";
import FormContainer from "../../components/FormContainer";
import { GoogleLogin, CredentialResponse } from "@react-oauth/google";

export default function LoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const router = useRouter();
  const { login } = useAuth();

  const googleLoginButtonRef = useRef<HTMLDivElement>(null);

  const handleCustomGoogleClick = () => {
    const googleButton =
      googleLoginButtonRef.current?.querySelector('div[role="button"]');
    if (googleButton instanceof HTMLElement) {
      googleButton.click();
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    try {
      const response = await axios.post("http://localhost:8080/login", {
        email: email,
        password: password,
      });

      login(response.data.token);

      router.push("/dashboard");
    } catch (err) {
      if (axios.isAxiosError(err) && err.response) {
        setError(err.response.data.error || "Login failed.");
      } else {
        setError("Login failed.");
      }
    }
  };
  const handleGoogleLoginSuccess = async (
    credentialResponse: CredentialResponse
  ) => {
    const idToken = credentialResponse.credential;
    if (!idToken) {
      setError("Google login failed.");
      return;
    }

    try {
      // Pozivamo naš NOVI backend endpoint
      const response = await axios.post("http://localhost:8080/auth/google", {
        token: idToken,
      });

      // Ostatak je isti kao i kod običnog logina
      login(response.data.token);
      router.push("/dashboard");
    } catch (err) {
      setError("Failed to log in with Google.");
    }
  };

  return (
    <FormContainer title="Login">
      {/* Forma za Email/Lozinku */}
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
          Sign In
        </button>
      </form>

      {/* Separator */}
      <div className="text-center my-6 uppercase tracking-wider text-ink/60">
        Or
      </div>

      {/* --- POČETAK PROMENA ZA GOOGLE DUGME --- */}
      <div className="w-full flex justify-center">
        {/* Naše custom, stilizovano dugme */}
        <button
          type="button"
          onClick={handleCustomGoogleClick}
          className="bg-canvas text-ink text-2xl uppercase font-bold py-3 px-8 rounded-md border-2 border-ink hover:bg-ink hover:text-canvas flex items-center space-x-3"
        >
          {/* Google SVG Logo */}
          <svg className="w-6 h-6" viewBox="0 0 48 48">
            <path
              fill="#FFC107"
              d="M43.611 20.083H42V20H24v8h11.303c-1.649 4.657-6.08 8-11.303 8c-6.627 0-12-5.373-12-12s5.373-12 12-12c3.059 0 5.842 1.154 7.961 3.039l5.657-5.657C34.046 6.053 29.268 4 24 4C12.955 4 4 12.955 4 24s8.955 20 20 20s20-8.955 20-20c0-1.341-.138-2.65-.389-3.917z"
            ></path>
            <path
              fill="#FF3D00"
              d="M6.306 14.691c-1.319 3.197-2.164 6.745-2.164 10.559C4.142 35.845 13.488 44 24 44c5.166 0 9.86-1.556 13.694-4.205l-5.657-5.657C30.046 36.686 27.218 38 24 38c-4.969 0-9.102-3.214-10.61-7.533z"
            ></path>
            <path
              fill="#4CAF50"
              d="M24 44c5.166 0 9.86-1.556 13.694-4.205l-5.657-5.657C30.046 36.686 27.218 38 24 38c-4.969 0-9.102-3.214-10.61-7.533L7.34 34.694C10.596 41.282 16.83 44 24 44z"
            ></path>
            <path
              fill="#1976D2"
              d="M43.611 20.083H42V20H24v8h11.303c-0.792 2.237-2.231 4.16-4.087 5.571l5.657 5.657C41.813 36.467 44 32.062 44 27.521c0-2.641-0.649-5.114-1.789-7.225z"
            ></path>
          </svg>
          <span>Sign in with Google</span>
        </button>
      </div>

      {/* Nevidljivo, pravo Google dugme */}
      <div
        ref={googleLoginButtonRef}
        className="opacity-0 absolute top-0 left-0 -z-10"
      >
        <GoogleLogin
          onSuccess={handleGoogleLoginSuccess}
          onError={() => {
            setError("Google Login Failed");
          }}
        />
      </div>
      {/* --- KRAJ PROMENA --- */}
    </FormContainer>
  );
}
