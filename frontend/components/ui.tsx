"use client";
import React from "react";

// Small building blocks shared by the account pages so they all look and behave the same.

interface FieldProps {
  id: string;
  label: string;
  type?: "text" | "email" | "password";
  value: string;
  onChange: (value: string) => void;
  autoComplete?: string;
  required?: boolean;
  minLength?: number;
  maxLength?: number;
  hint?: string;
}

export function Field({
  id,
  label,
  type = "text",
  value,
  onChange,
  autoComplete,
  required = true,
  minLength,
  maxLength,
  hint,
}: FieldProps) {
  return (
    <div className="w-full">
      <label className="block uppercase tracking-wider mb-1" htmlFor={id}>
        {label}
      </label>
      <input
        className="w-full p-3 bg-canvas border-2 border-ink rounded-md focus:outline-none"
        id={id}
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        autoComplete={autoComplete}
        required={required}
        minLength={minLength}
        maxLength={maxLength}
      />
      {hint && <p className="mt-1 text-base opacity-70">{hint}</p>}
    </div>
  );
}

type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement>;

export function PrimaryButton({ className = "", ...props }: ButtonProps) {
  return (
    <button
      {...props}
      className={`bg-ink text-canvas text-3xl uppercase font-bold py-3 px-12 rounded-md border-2 border-b-8 border-ink hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed ${className}`}
    />
  );
}

export function SecondaryButton({ className = "", ...props }: ButtonProps) {
  return (
    <button
      {...props}
      className={`bg-canvas text-ink text-xl uppercase font-bold py-2 px-6 rounded-md border-2 border-ink hover:bg-ink hover:text-canvas disabled:opacity-50 disabled:cursor-not-allowed ${className}`}
    />
  );
}

export function DangerButton({ className = "", ...props }: ButtonProps) {
  return (
    <button
      {...props}
      className={`bg-red-600 text-white text-xl uppercase font-bold py-2 px-6 rounded-md hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed ${className}`}
    />
  );
}

export function ErrorText({ children }: { children?: React.ReactNode }) {
  if (!children) return null;
  return (
    <p className="text-red-500 text-lg" role="alert">
      {children}
    </p>
  );
}

export function SuccessText({ children }: { children?: React.ReactNode }) {
  if (!children) return null;
  return (
    <p className="text-lg text-ink" role="status">
      {children}
    </p>
  );
}
