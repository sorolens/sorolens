"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import {
  createContractNote,
  deleteContractNote,
  getContractNotes,
} from "@/lib/api";
import type { ContractNote } from "@/lib/types";
import {
  parseMarkdown,
  type InlineToken,
  type MarkdownBlock,
} from "@/lib/markdown";

function renderInline(tokens: InlineToken[]) {
  return tokens.map((t, i) => {
    switch (t.type) {
      case "code":
        return (
          <code
            key={i}
            className="rounded bg-black/40 px-1 py-0.5 font-mono text-[0.85em]"
          >
            {t.value}
          </code>
        );
      case "strong":
        return (
          <strong key={i} className="font-semibold">
            {t.value}
          </strong>
        );
      case "em":
        return (
          <em key={i} className="italic">
            {t.value}
          </em>
        );
      case "link":
        return (
          <a
            key={i}
            href={t.href}
            target="_blank"
            rel="noopener noreferrer nofollow"
            className="text-[var(--color-accent)] underline underline-offset-2 hover:opacity-80"
          >
            {t.value}
          </a>
        );
      default:
        return t.value;
    }
  });
}

function MarkdownBlockView({ block }: { block: MarkdownBlock }) {
  switch (block.type) {
    case "heading": {
      const HeadingTag =
        block.level === 1 ? "h4" : block.level === 2 ? "h5" : "h6";
      return (
        <HeadingTag className="mt-4 mb-2 font-semibold text-[var(--color-text-primary)]">
          {renderInline(block.inline)}
        </HeadingTag>
      );
    }
    case "code":
      return (
        <pre className="my-2 overflow-x-auto rounded bg-black/40 p-3 text-xs">
          <code>{block.value}</code>
        </pre>
      );
    case "list": {
      const items = block.items.map((item, i) => (
        <li key={i} className="ml-5 list-disc">
          {renderInline(item)}
        </li>
      ));
      return <ul className="my-2 space-y-1 text-sm">{items}</ul>;
    }
    case "quote":
      return (
        <blockquote className="my-2 border-l-2 border-[var(--color-border)] pl-3 text-sm italic text-[var(--color-text-secondary)]">
          {renderInline(block.inline)}
        </blockquote>
      );
    default:
      return (
        <p className="my-2 text-sm leading-relaxed text-[var(--color-text-secondary)]">
          {renderInline(block.inline)}
        </p>
      );
  }
}

function NoteBody({ body }: { body: string }) {
  const blocks = parseMarkdown(body);
  return (
    <div>
      {blocks.map((block, i) => (
        <MarkdownBlockView key={i} block={block} />
      ))}
    </div>
  );
}

export function ContractNotes({ contractId }: { contractId: string }) {
  const [notes, setNotes] = useState<ContractNote[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [author, setAuthor] = useState("");
  const [body, setBody] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      setLoading(true);
      try {
        const data = await getContractNotes(contractId);
        if (!cancelled) {
          // Guard against a non-list payload (e.g. a mocked/older API).
          setNotes(Array.isArray(data?.notes) ? data.notes : []);
        }
      } catch {
        if (!cancelled) setError("Could not load notes.");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, [contractId]);

  const handleSubmit = useCallback(
    async (e: FormEvent<HTMLFormElement>) => {
      e.preventDefault();
      const trimmed = body.trim();
      if (!trimmed || submitting) return;
      setSubmitting(true);
      setError(null);
      try {
        const note = await createContractNote(contractId, {
          author: author.trim(),
          body: trimmed,
        });
        setNotes((prev) => [note, ...prev]);
        setBody("");
      } catch {
        setError("Could not save the note. Please try again.");
      } finally {
        setSubmitting(false);
      }
    },
    [author, body, contractId, submitting],
  );

  const handleDelete = useCallback(
    async (id: string) => {
      setError(null);
      try {
        await deleteContractNote(contractId, id);
        setNotes((prev) => prev.filter((n) => n.id !== id));
      } catch {
        setError("Could not delete the note. Please try again.");
      }
    },
    [contractId],
  );

  return (
    <div className="space-y-4">
      <form
        onSubmit={handleSubmit}
        className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4"
      >
        <div className="mb-3 flex flex-col gap-2 sm:flex-row">
          <input
            type="text"
            value={author}
            onChange={(e) => setAuthor(e.target.value)}
            placeholder="Your name (optional)"
            maxLength={120}
            className="rounded border border-[var(--color-border)] bg-transparent px-3 py-2 text-sm sm:w-56"
          />
        </div>
        <textarea
          value={body}
          onChange={(e) => setBody(e.target.value)}
          placeholder="Add a markdown note (institutional knowledge, migration context, runbooks…)"
          rows={4}
          maxLength={10000}
          className="w-full rounded border border-[var(--color-border)] bg-transparent px-3 py-2 text-sm"
        />
        <div className="mt-3 flex items-center justify-between">
          <span className="text-xs text-[var(--color-text-secondary)]">
            Markdown supported. HTML is escaped.
          </span>
          <button
            type="submit"
            disabled={submitting || body.trim().length === 0}
            className="rounded bg-[var(--color-accent)] px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50"
          >
            {submitting ? "Saving…" : "Add note"}
          </button>
        </div>
      </form>

      {error && (
        <p className="text-sm text-red-400" role="alert">
          {error}
        </p>
      )}

      {loading ? (
        <p className="text-sm text-[var(--color-text-secondary)]">
          Loading notes…
        </p>
      ) : notes.length === 0 ? (
        <p className="text-sm text-[var(--color-text-secondary)]">
          No notes yet. Add the first one.
        </p>
      ) : (
        <ul className="space-y-3">
          {notes.map((note) => (
            <li
              key={note.id}
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4"
            >
              <div className="mb-1 flex items-center justify-between gap-3">
                <div className="text-xs text-[var(--color-text-secondary)]">
                  <span className="font-medium text-[var(--color-text-primary)]">
                    {note.author}
                  </span>{" "}
                  · {new Date(note.created_at).toLocaleString()}
                </div>
                <button
                  type="button"
                  onClick={() => handleDelete(note.id)}
                  className="text-xs text-red-400 hover:opacity-80"
                  aria-label={`Delete note by ${note.author}`}
                >
                  Delete
                </button>
              </div>
              <NoteBody body={note.body} />
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
