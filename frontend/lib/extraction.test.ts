import { describe, expect, it } from "vitest";
import {
  cellText,
  cleanFields,
  csvFileName,
  fieldsProblem,
  MAX_FIELDS,
  reasonKey,
  safeCell,
  tableHeader,
  tableRows,
  TEMPLATES,
  templateFields,
  toCsv,
  toTsv,
  type FieldResult,
  type FieldSpec,
  type Row,
} from "./extraction";

const field = (name: string, type: FieldSpec["type"] = "text"): FieldSpec => ({ name, description: "", type });
const result = (over: Partial<FieldResult>): FieldResult => ({
  name: "x",
  type: "text",
  value: "v",
  quote: "q",
  page: 1,
  found: true,
  verified: true,
  reason: "",
  ...over,
});

describe("templates", () => {
  it("each have fields with unique names and valid types", () => {
    for (const template of TEMPLATES) {
      expect(template.fields.length).toBeGreaterThan(0);
      expect(template.fields.length).toBeLessThanOrEqual(MAX_FIELDS);
      expect(fieldsProblem(template.fields), template.id).toBeNull();
    }
  });

  it("start from copies, so editing a template does not change it", () => {
    const fields = templateFields("invoice");
    fields[0].name = "Changed";
    expect(templateFields("invoice")[0].name).toBe("Invoice number");
  });

  it("give one empty row for your own fields", () => {
    expect(templateFields("custom")).toEqual([{ name: "", description: "", type: "text" }]);
  });
});

describe("fieldsProblem", () => {
  it("accepts good fields", () => {
    expect(fieldsProblem([field("Total", "amount"), field("Date", "date")])).toBeNull();
  });

  it("says what is wrong", () => {
    expect(fieldsProblem([])).toBe("extract.errNoFields");
    expect(fieldsProblem([field("  ")])).toBe("extract.errNoName");
    expect(fieldsProblem([field("Total"), field("  total ")])).toBe("extract.errDuplicate");
    expect(fieldsProblem([field("Due  date"), field("Due date")])).toBe("extract.errDuplicate");
  });
});

describe("cleanFields", () => {
  it("tidies names and descriptions", () => {
    expect(cleanFields([{ name: "  Due   date ", description: " when\nit is due ", type: "date" }])).toEqual([
      { name: "Due date", description: "when it is due", type: "date" },
    ]);
  });
});

describe("the table", () => {
  const fields = [field("Invoice number"), field("Total", "amount")];
  const rows: Row[] = [
    { id: 1, name: "a.pdf", result: { name: "a.pdf", fields: [result({ value: "INV-1", quote: "Invoice INV-1" }), result({ found: false, value: "" })] } },
    { id: 2, name: "b.pdf", error: "failed" },
  ];

  it("has a heading per field, and per quote when asked", () => {
    expect(tableHeader(fields, "Document", false, "quote")).toEqual(["Document", "Invoice number", "Total"]);
    expect(tableHeader(fields, "Document", true, "quote")).toEqual(["Document", "Invoice number", "Invoice number (quote)", "Total", "Total (quote)"]);
  });

  it("has a row per document, with nothing for what was not found or failed", () => {
    expect(tableRows(rows, fields, false)).toEqual([
      ["a.pdf", "INV-1", ""],
      ["b.pdf", "", ""],
    ]);
    expect(tableRows(rows, fields, true)[0]).toEqual(["a.pdf", "INV-1", "Invoice INV-1", "", ""]);
  });

  it("writes only found values", () => {
    expect(cellText(result({ found: false, value: "stray" }))).toBe("");
    expect(cellText(undefined)).toBe("");
    expect(cellText(result({ value: "120 euros" }))).toBe("120 euros");
  });
});

describe("safeCell", () => {
  it("keeps a spreadsheet from running what a document says", () => {
    for (const text of ["=SUM(A1:A9)", "=HYPERLINK(\"http://evil\",\"x\")", "+1+1", "-2+3", "@SUM(1)", "\t=1", "\r=1", "-cmd|' /C calc'!A0"]) {
      expect(safeCell(text), text).toBe(`'${text}`);
    }
  });

  it("leaves ordinary text and plain numbers alone", () => {
    for (const text of ["Invoice 42", "120 euros", "-5", "+1.5", "1,020.00", "-1,5", "", "a=b", "100%"]) {
      expect(safeCell(text), text).toBe(text);
    }
  });
});

describe("toCsv", () => {
  it("quotes what needs it, doubles quotes, ends lines with CRLF", () => {
    const csv = toCsv([
      ["Document", "Notes"],
      ['a, "b".pdf', "line one\nline two"],
    ]);
    expect(csv).toBe('Document,Notes\r\n"a, ""b"".pdf","line one\nline two"\r\n');
  });

  it("is safe to open in a spreadsheet", () => {
    expect(toCsv([["=1+1", "ok"]])).toBe("'=1+1,ok\r\n");
    expect(toCsv([["=1,2"]])).toBe('"\'=1,2"\r\n');
  });

  it("handles an empty table", () => {
    expect(toCsv([])).toBe("\r\n");
  });
});

describe("toTsv", () => {
  it("separates with tabs and flattens line breaks", () => {
    expect(
      toTsv([
        ["a", "b\nc"],
        ["=x", "d\te"],
      ])
    ).toBe("a\tb c\n'=x\td e\n");
  });
});

describe("csvFileName", () => {
  it("is made from the first document", () => {
    expect(csvFileName("Invoice March 2026.pdf")).toBe("Invoice-March-2026.csv");
    expect(csvFileName("../../etc/passwd")).toBe("etc-passwd.csv");
    expect(csvFileName("")).toBe("extraction.csv");
    expect(csvFileName("???.pdf")).toBe("extraction.csv");
    expect(csvFileName("x".repeat(200) + ".pdf").length).toBeLessThanOrEqual(64);
  });
});

describe("reasonKey", () => {
  it("knows the reasons the server gives", () => {
    expect(reasonKey("The quote was not found in the document.")).toBe("extract.reasonQuote");
    expect(reasonKey("The value has a number that is not in the quote.")).toBe("extract.reasonNumber");
    expect(reasonKey("The quote comes from text that reads like an instruction to an AI.")).toBe("extract.reasonInstruction");
    expect(reasonKey("something new")).toBeNull();
  });
});
