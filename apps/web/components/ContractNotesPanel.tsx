"use client";

import { useCallback, useEffect, useState } from "react";
import {
  ApiError,
  createContractNote,
  deleteContractNote,
  listContractNotes,
  updateContractNote,
} from "@/lib/api";
import type { ContractNote } from "@/lib/types";
import { useSessionUserId } from "@/lib/useSession";
import { ContractNotes } from "./ContractNotes";

interface ContractNotesPanelProps {
  contractId: string;
}

/**
 * Contract notes (issue #164): owns the notes for one contract and applies the
 * create/update/delete calls, so the detail page only has to render
 * `<ContractNotesPanel contractId={id} />` (the same shape as HealthScoreCard
 * and SnapshotPanel).
 */
export function ContractNotesPanel({ contractId }: ContractNotesPanelProps) {
  const [notes, setNotes] = useState<ContractNote[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // A note's author must be the verified session user, both for creation and
  // for edits/deletes.
  const userId = useSessionUserId();

  useEffect(() => {
    let cancelled = false;

    async function load() {
      setLoading(true);
      try {
        const data = await listContractNotes(contractId);
        if (!cancelled) {
          setNotes(data.notes ?? []);
          setError(null);
        }
      } catch {
        if (!cancelled) setError("Failed to load notes.");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, [contractId]);

  const handleCreate = useCallback(
    async (body: string) => {
      setSaving(true);
      setError(null);
      try {
        const note = await createContractNote(contractId, body, userId);
        // The API returns notes newest first, so the new note goes on top
        // without a refetch.
        setNotes((prev) => [note, ...prev]);
      } catch (err) {
        setError(noteErrorMessage(err, "Failed to add the note."));
      } finally {
        setSaving(false);
      }
    },
    [contractId, userId]
  );

  const handleUpdate = useCallback(
    async (noteId: string, body: string) => {
      setSaving(true);
      setError(null);
      try {
        const updated = await updateContractNote(
          contractId,
          noteId,
          body,
          userId
        );
        setNotes((prev) => prev.map((n) => (n.id === noteId ? updated : n)));
      } catch (err) {
        setError(noteErrorMessage(err, "Failed to save the note."));
      } finally {
        setSaving(false);
      }
    },
    [contractId, userId]
  );

  const handleDelete = useCallback(
    async (noteId: string) => {
      setSaving(true);
      setError(null);
      try {
        await deleteContractNote(contractId, noteId, userId);
        setNotes((prev) => prev.filter((n) => n.id !== noteId));
      } catch (err) {
        setError(noteErrorMessage(err, "Failed to delete the note."));
      } finally {
        setSaving(false);
      }
    },
    [contractId, userId]
  );

  return (
    <ContractNotes
      notes={notes}
      loading={loading}
      saving={saving}
      error={error}
      currentUserId={userId}
      onCreate={handleCreate}
      onUpdate={handleUpdate}
      onDelete={handleDelete}
    />
  );
}

/** noteErrorMessage turns an API rejection into something the author can act on. */
function noteErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) {
    if (err.status === 401) {
      return "You need a contributor identity to write notes.";
    }
    if (err.status === 403) return "Only the author of a note may change it.";
    if (err.status === 404) return "That contract or note no longer exists.";
    if (err.status === 422) return "That note body was rejected by the API.";
  }
  return fallback;
}
