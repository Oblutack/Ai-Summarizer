export interface Document {
  ID: number;
  CreatedAt: string;
  Filename: string;
  Summary: string;
  hasContent?: boolean;
}

// A passage of the document that an answer cites.
export interface ChatSource {
  id: number;
  text: string;
  // Pages are counted within each file; null when the document has no page information.
  page: number | null;
  pageEnd: number | null;
  // The file name, when several files were summarized together.
  document?: string;
}

export interface ChatMessage {
  role: "user" | "assistant";
  content: string;
  // Only on assistant messages: the passages the answer cites.
  sources?: ChatSource[];
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
