"use client"; // Ova komponenta je klijentska

import { AuthProvider } from '../contexts/AuthContext';
import { GoogleOAuthProvider } from '@react-oauth/google';
import React from 'react';

export default function Providers({ children }: { children: React.ReactNode }) {
    const clientId = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID || "";

    return (
        <GoogleOAuthProvider clientId={clientId}>
            <AuthProvider>
                {children}
            </AuthProvider>
        </GoogleOAuthProvider>
    );
}