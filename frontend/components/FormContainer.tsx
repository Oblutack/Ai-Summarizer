import React from "react";

interface FormContainerProps {
  title: string;
  children: React.ReactNode;
}

export default function FormContainer({ title, children }: FormContainerProps) {
  return (
    <div className="max-w-xl mx-auto mt-12 border-2 border-ink rounded-lg p-8">
      <h1 className="text-4xl uppercase tracking-widest text-center mb-8">
        {title}
      </h1>
      {children}
    </div>
  );
}
