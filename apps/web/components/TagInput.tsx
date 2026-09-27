"use client";

import { useState } from "react";

// Mirrors the API's tag validation: 1-32 chars, a-z/0-9/-/_ and must start
// with a letter or digit.
const TAG_RE = /^[a-z0-9][a-z0-9_-]{0,31}$/;

const INVALID_TAG_MESSAGE =
  "Tags use 1-32 characters: a-z, 0-9, - and _, starting with a letter or digit.";

interface TagInputProps {
  /** Current tags for the contract. */
  tags: string[];
  /** Called with a normalized, valid, not-yet-present tag. */
  onAdd: (tag: string) => void;
  /** Called with a tag to remove. */
  onRemove: (tag: string) => void;
  /** Disables editing (e.g. while a request is in flight). */
  disabled?: boolean;
  /** Server-side error to surface under the input. */
  error?: string | null;
  placeholder?: string;
}

/**
 * A lightweight chip input for contract tags. Type a tag and press Enter (or
 * comma) to add it; press the × on a chip (or Backspace on an empty input) to
 * remove it. Purely presentational: the parent owns the tag list and performs
 * the API calls.
 */
export function TagInput({
  tags,
  onAdd,
  onRemove,
  disabled = false,
  error = null,
  placeholder = "Add a tag…",
}: TagInputProps) {
  const [value, setValue] = useState("");
  const [inputError, setInputError] = useState<string | null>(null);

  const commit = () => {
    const tag = value.trim().toLowerCase();
    if (!tag) return;
    if (!TAG_RE.test(tag)) {
      setInputError(INVALID_TAG_MESSAGE);
      return;
    }
    setInputError(null);
    setValue("");
    // Adding an existing tag is a no-op; don't bounce it off the API.
    if (!tags.includes(tag)) onAdd(tag);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      commit();
    } else if (e.key === "Backspace" && value === "" && tags.length > 0) {
      onRemove(tags[tags.length - 1]);
    }
  };

  const message = inputError ?? error;

  return (
    <div>
      <div className="flex flex-wrap items-center gap-1.5">
        {tags.map((tag) => (
          <span
            key={tag}
            data-testid="tag-chip"
            className="inline-flex items-center gap-1 rounded-full bg-[var(--color-accent)]/15 px-2.5 py-0.5 text-xs font-medium text-[var(--color-accent)]"
          >
            {tag}
            <button
              type="button"
              aria-label={`Remove tag ${tag}`}
              disabled={disabled}
              onClick={() => onRemove(tag)}
              className="rounded-full leading-none opacity-70 transition-opacity hover:opacity-100 disabled:cursor-not-allowed"
            >
              ×
            </button>
          </span>
        ))}
        <input
          id="tag-input"
          data-testid="tag-input"
          type="text"
          value={value}
          disabled={disabled}
          onChange={(e) => {
            setValue(e.target.value);
            setInputError(null);
          }}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          aria-label="Add tag"
          autoComplete="off"
          spellCheck={false}
          className="min-w-[8rem] flex-1 rounded-lg border border-[var(--color-border)] bg-black/30 px-3 py-1.5 text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
        />
      </div>
      {message && (
        <p role="alert" className="mt-1.5 text-xs text-red-400">
          {message}
        </p>
      )}
    </div>
  );
}
