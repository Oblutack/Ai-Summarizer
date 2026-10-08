// A web link pasted into the text box: when the whole box is one address, the page behind it is what
// gets summarized (pasting a bare address as "text" would only summarize the address itself).

const MAX_LENGTH = 2048;

// The address if the text is one web link, otherwise null. Only http(s) links and "www." addresses count,
// so a file name or a sentence with a dot in it is never mistaken for a link.
export function webLinkIn(text: string): string | null {
  const value = text.trim();
  if (!value || value.length > MAX_LENGTH || /\s/.test(value)) return null;
  const candidate = /^https?:\/\//i.test(value) ? value : /^www\.[^./]+\.[^./]/i.test(value) ? `https://${value}` : null;
  if (!candidate) return null;
  try {
    const url = new URL(candidate);
    return url.hostname.includes(".") ? candidate : null;
  } catch {
    return null;
  }
}

// The file types that can be summarized, as the extensions the file picker offers.
export const DOCUMENT_EXTENSIONS = [".pdf", ".docx", ".pptx"] as const;

export function isDocumentFile(name: string): boolean {
  const lower = name.toLowerCase();
  return DOCUMENT_EXTENSIONS.some((extension) => lower.endsWith(extension));
}
