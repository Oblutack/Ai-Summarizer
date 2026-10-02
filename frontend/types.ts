export interface Document {
  ID: number;
  CreatedAt: string;
  Filename: string;
  Summary: string;
  hasContent?: boolean;
}

export interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

export interface User {
  id: number;
  email: string;
  emailVerified: boolean;
  // False for accounts created through Google sign-in until they set a password.
  hasPassword: boolean;
}

export interface SessionInfo {
  id: number;
  current: boolean;
  userAgent: string;
  createdAt: string;
  lastUsedAt: string;
}

export interface Usage {
  summaries: { used: number; limit: number };
  chats: { used: number; limit: number };
  resetsAt: string;
}
