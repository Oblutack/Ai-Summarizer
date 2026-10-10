"use client";
import { useEffect, useId, useRef, useState } from "react";
import axios from "axios";
import { API_URL, apiError } from "../lib/api";
import {
  cleanFields,
  csvFileName,
  CUSTOM_TEMPLATE,
  FIELD_TYPES,
  fieldsProblem,
  MAX_DOCUMENTS,
  MAX_FIELDS,
  MAX_NAME,
  tableHeader,
  tableRows,
  TEMPLATES,
  templateFields,
  toCsv,
  toTsv,
  type ExtractionResult,
  type FieldSpec,
  type FieldType,
  type Row,
} from "../lib/extraction";
import type { Document } from "../types";
import CopyButton from "./CopyButton";
import ExtractTable from "./ExtractTable";
import { useT } from "./I18nProvider";
import { actionButton, fieldClass, textButton } from "./styles";

interface TagCount {
  tag: string;
  count: number;
}

// Pulls named fields out of a saved document (or out of every document with a tag) into a table, with the exact quote
// each value was read from and whether that checks out. The table can be copied or downloaded as CSV.
export default function ExtractPanel({ doc }: { doc: Document }) {
  const t = useT();
  const ids = { template: useId(), tag: useId(), quotes: useId() };
  const [templateId, setTemplateId] = useState("invoice");
  const [fields, setFields] = useState<FieldSpec[]>(() => templateFields("invoice"));
  const [scope, setScope] = useState<"this" | "tag">("this");
  const [tags, setTags] = useState<TagCount[]>([]);
  const [tag, setTag] = useState("");
  const [includeQuotes, setIncludeQuotes] = useState(false);
  const [rows, setRows] = useState<Row[]>([]);
  // The fields the shown table was made with (editing the fields afterwards must not change its columns).
  const [shownFields, setShownFields] = useState<FieldSpec[]>([]);
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const alive = useRef(true);

  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  useEffect(() => {
    axios
      .get<TagCount[]>(`${API_URL}/documents/tags`)
      .then((response) => {
        if (!alive.current) return;
        setTags(response.data);
        setTag((current) => current || response.data[0]?.tag || "");
      })
      .catch(() => undefined);
  }, []);

  const chooseTemplate = (id: string) => {
    setTemplateId(id);
    setFields(templateFields(id));
  };

  const edit = (index: number, patch: Partial<FieldSpec>) => {
    setFields((current) => current.map((f, i) => (i === index ? { ...f, ...patch } : f)));
    setTemplateId(CUSTOM_TEMPLATE);
  };

  const run = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setNotice("");
    const problem = fieldsProblem(fields);
    if (problem) {
      setError(t(problem));
      return;
    }
    const specs = cleanFields(fields);
    setRows([]);
    setShownFields(specs);
    setProgress({ done: 0, total: 1 });

    try {
      let targets: { id: number; name: string }[] = [{ id: doc.ID, name: doc.Filename }];
      if (scope === "tag") {
        const response = await axios.get<Document[]>(`${API_URL}/documents?tag=${encodeURIComponent(tag)}&limit=${MAX_DOCUMENTS}`);
        targets = response.data.filter((d) => d.hasContent).slice(0, MAX_DOCUMENTS).map((d) => ({ id: d.ID, name: d.Filename }));
        if (targets.length === 0) {
          setError(t("extract.errNoDocs"));
          setProgress(null);
          return;
        }
      }
      setProgress({ done: 0, total: targets.length });
      for (const target of targets) {
        if (!alive.current) return;
        try {
          const response = await axios.post<ExtractionResult>(`${API_URL}/documents/${target.id}/extract`, { fields: specs });
          if (!alive.current) return;
          setRows((current) => [...current, { id: target.id, name: target.name, result: response.data }]);
        } catch (err) {
          if (!alive.current) return;
          if (axios.isAxiosError(err) && err.response?.status === 429) {
            setNotice(t("extract.stoppedQuota"));
            break;
          }
          setRows((current) => [...current, { id: target.id, name: target.name, error: apiError(err, t("extract.failed")) }]);
        }
        setProgress((p) => (p ? { ...p, done: p.done + 1 } : p));
      }
    } catch (err) {
      if (alive.current) setError(apiError(err, t("extract.failed")));
    } finally {
      if (alive.current) setProgress(null);
    }
  };

  const table = () => [tableHeader(shownFields, t("extract.colDocument"), includeQuotes, t("extract.quote")), ...tableRows(rows, shownFields, includeQuotes)];

  const download = () => {
    const blob = new Blob(["﻿" + toCsv(table())], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = csvFileName(rows[0]?.name ?? doc.Filename);
    link.click();
    URL.revokeObjectURL(url);
  };

  const suspicious = rows.filter((row) => row.result?.suspicious).map((row) => row.name);
  const busy = progress !== null;
  const done = rows.length > 0 && !busy;

  return (
    <div className="mt-4 rounded-xl border border-ink/20 bg-canvas/50 p-4" data-testid="extract">
      <p className="text-sm text-ink/70">{t("extract.intro")}</p>
      <form onSubmit={run} className="mt-3 space-y-4">
        <div className="flex flex-col gap-1">
          <label htmlFor={ids.template} className="label mb-0">
            {t("extract.template")}
          </label>
          <select id={ids.template} className={`${fieldClass} max-w-xs`} value={templateId} onChange={(e) => chooseTemplate(e.target.value)}>
            {TEMPLATES.map((tpl) => (
              <option key={tpl.id} value={tpl.id}>
                {t(tpl.label)}
              </option>
            ))}
            <option value={CUSTOM_TEMPLATE}>{t("extract.templateCustom")}</option>
          </select>
        </div>

        <fieldset className="space-y-2">
          <legend className="label">{t("extract.fieldsLegend")}</legend>
          <ul className="space-y-2">
            {fields.map((f, i) => (
              <li key={i} className="flex flex-wrap items-end gap-2" data-testid="extract-field">
                <div className="flex min-w-[10rem] flex-1 flex-col gap-1">
                  <label htmlFor={`${ids.template}-n${i}`} className="text-sm font-medium">
                    {t("extract.fieldName")}
                  </label>
                  <input id={`${ids.template}-n${i}`} className={fieldClass} value={f.name} maxLength={MAX_NAME} onChange={(e) => edit(i, { name: e.target.value })} autoComplete="off" />
                </div>
                <div className="flex min-w-[10rem] flex-[2] flex-col gap-1">
                  <label htmlFor={`${ids.template}-d${i}`} className="text-sm font-medium">
                    {t("extract.fieldDescription")}
                  </label>
                  <input id={`${ids.template}-d${i}`} className={fieldClass} value={f.description} maxLength={200} onChange={(e) => edit(i, { description: e.target.value })} autoComplete="off" />
                </div>
                <div className="flex flex-col gap-1">
                  <label htmlFor={`${ids.template}-t${i}`} className="text-sm font-medium">
                    {t("extract.fieldType")}
                  </label>
                  <select id={`${ids.template}-t${i}`} className={fieldClass} value={f.type} onChange={(e) => edit(i, { type: e.target.value as FieldType })}>
                    {FIELD_TYPES.map((ft) => (
                      <option key={ft.value} value={ft.value}>
                        {t(ft.label)}
                      </option>
                    ))}
                  </select>
                </div>
                <button
                  type="button"
                  onClick={() => {
                    setFields((current) => current.filter((_, at) => at !== i));
                    setTemplateId(CUSTOM_TEMPLATE);
                  }}
                  className="flex h-10 w-10 items-center justify-center rounded-md text-2xl leading-none text-ink/70 hover:bg-ink/10 hover:text-danger"
                  aria-label={t("extract.removeField", { n: i + 1 })}
                  title={t("extract.removeField", { n: i + 1 })}
                >
                  &times;
                </button>
              </li>
            ))}
          </ul>
          <div className="flex flex-wrap items-center gap-3">
            <button
              type="button"
              className={textButton}
              disabled={fields.length >= MAX_FIELDS}
              onClick={() => {
                setFields((current) => [...current, { name: "", description: "", type: "text" }]);
                setTemplateId(CUSTOM_TEMPLATE);
              }}
            >
              {t("extract.addField")}
            </button>
            {fields.length >= MAX_FIELDS && <span className="muted text-sm">{t("extract.limit", { max: MAX_FIELDS })}</span>}
          </div>
        </fieldset>

        <fieldset className="space-y-1">
          <legend className="label">{t("extract.scopeLegend")}</legend>
          <label className="flex items-center gap-2">
            <input type="radio" name={`scope-${doc.ID}`} checked={scope === "this"} onChange={() => setScope("this")} />
            {t("extract.scopeThis")}
          </label>
          <label className="flex items-center gap-2">
            <input type="radio" name={`scope-${doc.ID}`} checked={scope === "tag"} onChange={() => setScope("tag")} disabled={tags.length === 0} />
            {t("extract.scopeTag")}
          </label>
          {tags.length === 0 && <p className="muted text-sm">{t("extract.noTags")}</p>}
          {scope === "tag" && tags.length > 0 && (
            <div className="ml-6 flex flex-col gap-1">
              <label htmlFor={ids.tag} className="text-sm font-medium">
                {t("extract.tagPick")}
              </label>
              <select id={ids.tag} className={`${fieldClass} max-w-xs`} value={tag} onChange={(e) => setTag(e.target.value)}>
                {tags.map((tc) => (
                  <option key={tc.tag} value={tc.tag}>
                    {tc.tag} ({tc.count})
                  </option>
                ))}
              </select>
              <p className="muted text-sm">{t("extract.tagNote", { max: MAX_DOCUMENTS })}</p>
            </div>
          )}
        </fieldset>

        <label className="flex items-center gap-2" htmlFor={ids.quotes}>
          <input id={ids.quotes} type="checkbox" checked={includeQuotes} onChange={(e) => setIncludeQuotes(e.target.checked)} />
          {t("extract.includeQuotes")}
        </label>

        <button type="submit" disabled={busy} className={actionButton}>
          {busy ? t("extract.running", { done: progress?.done ?? 0, total: progress?.total ?? 1 }) : t("extract.run")}
        </button>
      </form>

      {error && (
        <p className="mt-3 font-semibold text-danger" role="alert">
          {error}
        </p>
      )}
      {busy && (
        <p className="mt-3 text-sm font-semibold text-ink/70" role="status">
          {t("extract.running", { done: progress?.done ?? 0, total: progress?.total ?? 1 })}
        </p>
      )}
      {notice && (
        <p className="mt-3 font-semibold text-ink/80" role="status">
          {notice}
        </p>
      )}

      {suspicious.length > 0 && (
        <p className="mt-3 rounded-lg border border-danger/60 bg-surface p-3 font-medium" role="note" data-testid="extract-suspicious">
          {t("extract.suspicious", { names: suspicious.join(", ") })}
        </p>
      )}

      {rows.length > 0 && (
        <section className="mt-4 space-y-3" aria-label={t("extract.resultTitle")} data-testid="extraction">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h3 className="text-xl font-bold">{t("extract.resultTitle")}</h3>
            {done && (
              <div className="flex flex-wrap items-center gap-2">
                <button type="button" onClick={download} className="btn btn-secondary btn-sm">
                  {t("extract.download")}
                </button>
                <CopyButton text={() => toTsv(table())} what={t("extract.what")} />
              </div>
            )}
          </div>
          <p className="muted text-sm">{t("extract.legend")}</p>
          <ExtractTable rows={rows} fields={shownFields} />
        </section>
      )}
    </div>
  );
}
