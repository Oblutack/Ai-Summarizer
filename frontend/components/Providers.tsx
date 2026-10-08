"use client"; // client component: providers hold state and use browser APIs

import { AuthProvider } from '../contexts/AuthContext';
import { GoogleOAuthProvider } from '@react-oauth/google';
import React from 'react';
import I18nProvider from './I18nProvider';

export default function Providers({ children, nonce }: { children: React.ReactNode; nonce?: string }) {
    const clientId = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID || "";

    return (
        <I18nProvider>
            <GoogleOAuthProvider clientId={clientId} nonce={nonce}>
                <AuthProvider>
                    {children}
                </AuthProvider>
            </GoogleOAuthProvider>
        </I18nProvider>
    );
}