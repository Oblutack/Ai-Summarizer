// Structured extraction: named fields pulled out of documents into a table.

import type { MessageKey } from "./i18n";

export type FieldType = "text" | "number" | "date" | "amount" | "list";

export interface FieldSpec {
  name: string;
  description: string;
  type: FieldType;
}

// One field's outcome, as the server sends it. `verified` means the quote is in the document and the value is in the
// quote; `reason` says why not otherwise.
export interface FieldResult {
  name: string;
  type: FieldType;
  value: string;
  quote: string;
  page: number | null;
  found: boolean;
  verified: boolean;
  reason: string;
}

export interface ExtractionResult {
  name: string;
  fields: FieldResult[];
  // The document has sentences written for an AI ("ignore your instructions..."): its values deserve a second look.
  suspicious?: boolean;
}

// One row of the table: a document, and either what was found in it or why that failed.
export interface Row {
  id: number;
  name: string;
  result?: ExtractionResult;
  error?: string;
}

// Keep in step with go-api (extractController.go) and python-ai-service (extraction.py).
export const MAX_FIELDS = 20;
export const MAX_NAME = 60;
export const MAX_DESCRIPTION = 200;
// How many documents one run may cover (each is one use of the daily allowance).
export const MAX_DOCUMENTS = 10;

export const FIELD_TYPES: { value: FieldType; label: MessageKey }[] = [
  { value: "text", label: "extract.typeText" },
  { value: "number", label: "extract.typeNumber" },
  { value: "date", label: "extract.typeDate" },
  { value: "amount", label: "extract.typeAmount" },
  { value: "list", label: "extract.typeList" },
];

const field = (name: string, type: FieldType = "text", description = ""): FieldSpec => ({ name, description, type });

// Starting points. The field names are the column headings of the table, so they stay in English.
export const TEMPLATES: { id: string; label: MessageKey; fields: FieldSpec[] }[] = [
  {
    id: "invoice",
    label: "extract.tInvoice",
    fields: [
      field("Invoice number"),
      field("Invoice date", "date"),
      field("Due date", "date"),
      field("Supplier"),
      field("Customer"),
      field("Total amount", "amount", "the total to pay, with VAT"),
      field("VAT", "amount"),
    ],
  },
  {
    id: "contract",
    label: "extract.tContract",
    fields: [
      field("Parties", "list"),
      field("Start date", "date"),
      field("Term", "text", "how long it runs"),
      field("Fee", "amount"),
      field("Payment terms"),
      field("Notice period", "text", "the notice needed to end it"),
      field("Governing law"),
    ],
  },
  {
    id: "receipt",
    label: "extract.tReceipt",
    fields: [field("Merchant"), field("Date", "date"), field("Total", "amount"), field("Payment method"), field("Items", "list")],
  },
  {
    id: "meeting",
    label: "extract.tMeeting",
    fields: [field("Date", "date"), field("Attendees", "list"), field("Decisions", "list"), field("Action items", "list", "who does what, by when")],
  },
];

export const CUSTOM_TEMPLATE = "custom";

export function templateFields(id: string): FieldSpec[] {
  const found = TEMPLATES.find((t) => t.id === id);
  return found ? found.fields.map((f) => ({ ...f })) : [field("")];
}

// Why a list of fields cannot be run yet, or null when it can.
export function fieldsProblem(fields: FieldSpec[]): MessageKey | null {
  if (fields.length === 0) return "extract.errNoFields";
  const seen = new Set<string>();
  for (const f of fields) {
    const name = f.name.trim().replace(/\s+/g, " ");
    if (!name) return "extract.errNoName";
    if (seen.has(name.toLowerCase())) return "extract.errDuplicate";
    seen.add(name.toLowerCase());
  }
  return null;
}

// The fields as they are sent: names and descriptions tidied, nothing else.
export function cleanFields(fields: FieldSpec[]): FieldSpec[] {
  return fields.map((f) => ({
    name: f.name.trim().replace(/\s+/g, " "),
    description: f.description.trim().replace(/\s+/g, " "),
    type: f.type,
  }));
}

// ---- the table ----------------------------------------------------------------------------------------------

// What is written for a field in a table: the value, or nothing when it was not found.
export function cellText(result: FieldResult | undefined): string {
  return result && result.found ? result.value : "";
}

export function tableHeader(fields: FieldSpec[], documentLabel: string, includeQuotes: boolean, quoteLabel: string): string[] {
  const header = [documentLabel];
  for (const f of fields) {
    header.push(f.name);
    if (includeQuotes) header.push(`${f.name} (${quoteLabel})`);
  }
  return header;
}

export function tableRows(rows: Row[], fields: FieldSpec[], includeQuotes: boolean): string[][] {
  return rows.map((row) => {
    const cells = [row.name];
    fields.forEach((f, i) => {
      const result = row.result?.fields[i];
      cells.push(cellText(result));
      if (includeQuotes) cells.push(result && result.found ? result.quote : "");
    });
    return cells;
  });
}

// A spreadsheet runs what starts with = + - @ (or a tab or return) as a formula, so a document that says
// "=HYPERLINK(...)" must not become one when its table is opened. Such a cell is written with a leading apostrophe,
// which a spreadsheet shows as nothing. A plain number such as -5 or +1.5 is left alone.
export function safeCell(text: string): string {
  if (/^[+-]?\d[\d.,]*$/.test(text)) return text;
  return /^[=+\-@\t\r]/.test(text) ? `'${text}` : text;
}

function csvCell(text: string): string {
  const safe = safeCell(text);
  return /[",\r\n]/.test(safe) ? `"${safe.replace(/"/g, '""')}"` : safe;
}

// A CSV file's text: comma separated, quotes doubled, lines ended with CRLF (RFC 4180).
export function toCsv(table: string[][]): string {
  return table.map((line) => line.map(csvCell).join(",")).join("\r\n") + "\r\n";
}

// The table for pasting into a spreadsheet: tab separated, no quoting needed because tabs and line breaks inside a
// cell are turned into spaces.
export function toTsv(table: string[][]): string {
  return table.map((line) => line.map((cell) => safeCell(cell).replace(/[\t\r\n]+/g, " ")).join("\t")).join("\n") + "\n";
}

// A file name for the download, from the first document's name.
export function csvFileName(firstName: string): string {
  const base = firstName.replace(/\.[^./\\]{1,5}$/, "").replace(/[^\p{L}\p{N}._ -]+/gu, " ").trim().replace(/\s+/g, "-").replace(/^[.\-_]+/, "").slice(0, 60);
  return `${base || "extraction"}.csv`;
}

// The reasons the server gives, as the keys of their translations.
const REASONS: Record<string, MessageKey> = {
  "The quote was not found in the document.": "extract.reasonQuote",
  "The value has a number that is not in the quote.": "extract.reasonNumber",
  "The value was not found in the quote.": "extract.reasonValue",
  "No quote was given to check the value against.": "extract.reasonNoQuote",
  "The quote comes from text that reads like an instruction to an AI.": "extract.reasonInstruction",
};

export function reasonKey(reason: string): MessageKey | null {
  return REASONS[reason] ?? null;
}
