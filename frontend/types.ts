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
