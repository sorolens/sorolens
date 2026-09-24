"use client";

/**
 * Contract tracking wizard (issue #140).
 *
 * Replaces the single-field "Track contract" modal with a guided three-step
 * flow so a new user hitting an empty dashboard has somewhere to start:
 *
 *   1. Identify  — paste the contract id, choose the network
 *   2. Validate   — the API checks the id's StrKey checksum and reports whether
 *                   it is already tracked
 *   3. Configure  — optional label, optional watchdog follow-up
 *
 * Creation reuses `POST /api/v1/contracts` and redirects to /contracts/[id].
 */

import { useCallback, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ApiError, trackContract, validateContract } from "@/lib/api";
import { ALL_NETWORKS, NETWORKS } from "@/lib/network";
import { getUserId } from "@/lib/user";

/** Networks a contract can actually live on. */
const CONCRETE_NETWORKS = NETWORKS.filter((n) => n !== ALL_NETWORKS);

/** Client-side shape check; the API performs the authoritative StrKey check. */
const CONTRACT_ID_RE = /^C[A-Z0-9]{55}$/;

const STEPS = [
  { key: "identify", label: "Identify", hint: "Which contract?" },
  { key: "validate", label: "Validate", hint: "Does it exist?" },
  { key: "configure", label: "Configure", hint: "Label and monitoring" },
] as const;

type Validation =
  | { status: "idle" }
  | { status: "checking" }
  | { status: "valid" }
  | { status: "tracked"; label: string | null }
  | { status: "invalid"; reason: string };

export default function NewContractPage() {
  const router = useRouter();

  const [step, setStep] = useState(0);
  const [contractId, setContractId] = useState("");
  const [network, setNetwork] = useState<string>("testnet");
  const [label, setLabel] = useState("");
  const [watchdog, setWatchdog] = useState(false);
  const [validation, setValidation] = useState<Validation>({ status: "idle" });
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const trimmedId = contractId.trim().toUpperCase();

  const idError = useMemo(() => {
    if (contractId.length === 0) return null;
    if (!CONTRACT_ID_RE.test(trimmedId)) {
      return "A contract id is 56 characters, starts with 'C', and uses A–Z and 0–9 only.";
    }
    return null;
  }, [contractId, trimmedId]);

  const labelError =
    label.length > 64 ? "Label must be 64 characters or fewer." : null;

  const stepOneValid =
    trimmedId.length > 0 && !idError && CONCRETE_NETWORKS.includes(network as never);

  // ---- step 2: ask the API ---------------------------------------------------

  const runValidation = useCallback(async () => {
    setValidation({ status: "checking" });
    setError(null);
    try {
      const res = await validateContract({
        contract_id: trimmedId,
        network,
      });
      if (!res.valid) {
        setValidation({
          status: "invalid",
          reason: res.reason ?? "This contract id could not be validated.",
        });
        return;
      }
      if (res.already_tracked) {
        setValidation({ status: "tracked", label: res.label });
        return;
      }
      setValidation({ status: "valid" });
    } catch (err) {
      setValidation({
        status: "invalid",
        reason:
          err instanceof ApiError
            ? err.message
            : "Could not reach the API to validate this contract.",
      });
    }
  }, [trimmedId, network]);

  const handleNext = useCallback(async () => {
    if (step === 0) {
      if (!stepOneValid) return;
      setStep(1);
      await runValidation();
      return;
    }
    if (step === 1) {
      if (validation.status !== "valid") return;
      setStep(2);
    }
  }, [step, stepOneValid, validation.status, runValidation]);

  const handleBack = useCallback(() => {
    setError(null);
    setStep((s) => Math.max(0, s - 1));
  }, []);

  // ---- step 3: create --------------------------------------------------------

  const handleCreate = useCallback(async () => {
    if (labelError) return;
    setSubmitting(true);
    setError(null);
    try {
      await trackContract(
        { id: trimmedId, label: label.trim() || undefined, network },
        getUserId(),
      );
      // Watchdog enrollment is recorded by the on-chain sorolens-watchdog
      // contract, so when the user asked for it we land them on the Watchdog
      // page to complete registration. Otherwise go straight to the contract.
      router.push(watchdog ? "/watchdog" : `/contracts/${trimmedId}`);
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : "An unexpected error occurred while creating the contract.",
      );
      setSubmitting(false);
    }
  }, [labelError, trimmedId, label, network, watchdog, router]);

  const nextEnabled =
    step === 0
      ? stepOneValid
      : step === 1
        ? validation.status === "valid"
        : !labelError;

  return (
    <div className="mx-auto max-w-2xl">
      <Link
        href="/contracts"
        className="text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
      >
        ← Back to contracts
      </Link>

      <h1 className="mt-4 text-2xl font-bold tracking-tight">Track a contract</h1>
      <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
        Three steps: identify the contract, confirm it, then label it.
      </p>

      {/* Progress */}
      <ol
        className="mt-6 flex items-center gap-2"
        aria-label="Progress"
        data-testid="wizard-progress"
      >
        {STEPS.map((s, i) => {
          const state = i < step ? "done" : i === step ? "current" : "upcoming";
          return (
            <li key={s.key} className="flex flex-1 flex-col gap-1">
              <span
                aria-current={state === "current" ? "step" : undefined}
                className={[
                  "h-1 rounded-full",
                  state === "upcoming"
                    ? "bg-[var(--color-border)]"
                    : "bg-[var(--color-accent)]",
                ].join(" ")}
              />
              <span
                className={[
                  "text-xs",
                  state === "current"
                    ? "font-semibold text-[var(--color-text-primary)]"
                    : "text-[var(--color-text-secondary)]",
                ].join(" ")}
              >
                {i + 1}. {s.label}
              </span>
            </li>
          );
        })}
      </ol>

      <div className="mt-6 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-6">
        {/* ---- Step 1: identify ---- */}
        {step === 0 && (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">{STEPS[0].hint}</h2>
            <div>
              <label
                htmlFor="wizard-contract-id"
                className="block text-sm font-medium"
              >
                Contract ID
              </label>
              <input
                id="wizard-contract-id"
                data-testid="wizard-contract-id"
                value={contractId}
                onChange={(e) => setContractId(e.target.value)}
                placeholder="C……"
                autoComplete="off"
                spellCheck={false}
                aria-invalid={idError ? true : undefined}
                aria-describedby={idError ? "wizard-contract-id-error" : undefined}
                className="mt-1 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-page)] px-3 py-2 font-mono text-sm focus:border-[var(--color-accent)] focus:outline-none"
              />
              {idError && (
                <p
                  id="wizard-contract-id-error"
                  data-testid="wizard-contract-id-error"
                  className="mt-1 text-xs text-[var(--color-danger)]"
                >
                  {idError}
                </p>
              )}
            </div>

            <div>
              <label htmlFor="wizard-network" className="block text-sm font-medium">
                Network
              </label>
              <select
                id="wizard-network"
                data-testid="wizard-network"
                value={network}
                onChange={(e) => setNetwork(e.target.value)}
                className="mt-1 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-page)] px-3 py-2 text-sm focus:border-[var(--color-accent)] focus:outline-none"
              >
                {CONCRETE_NETWORKS.map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </div>
          </div>
        )}

        {/* ---- Step 2: validate ---- */}
        {step === 1 && (
          <div className="space-y-4" data-testid="wizard-step-validate">
            <h2 className="text-lg font-semibold">Confirm the contract</h2>
            <dl className="text-sm">
              <dt className="text-[var(--color-text-secondary)]">Contract</dt>
              <dd className="break-all font-mono">{trimmedId}</dd>
              <dt className="mt-3 text-[var(--color-text-secondary)]">Network</dt>
              <dd>{network}</dd>
            </dl>

            {validation.status === "checking" && (
              <p
                role="status"
                data-testid="wizard-validating"
                className="text-sm text-[var(--color-text-secondary)]"
              >
                Checking the contract id…
              </p>
            )}

            {validation.status === "valid" && (
              <p
                role="status"
                data-testid="wizard-valid"
                className="rounded-md border border-[var(--color-safe)]/40 bg-[var(--color-safe)]/10 px-3 py-2 text-sm text-[var(--color-safe)]"
              >
                Contract id is valid. Continue to label it.
              </p>
            )}

            {validation.status === "invalid" && (
              <div
                role="alert"
                data-testid="wizard-invalid"
                className="rounded-md border border-[var(--color-danger)]/40 bg-[var(--color-danger)]/10 px-3 py-2 text-sm text-[var(--color-danger)]"
              >
                <p>{validation.reason}</p>
                <button
                  type="button"
                  onClick={runValidation}
                  className="mt-2 underline"
                >
                  Try again
                </button>
              </div>
            )}

            {validation.status === "tracked" && (
              <div
                role="status"
                data-testid="wizard-already-tracked"
                className="rounded-md border border-[var(--color-warning)]/40 bg-[var(--color-warning)]/10 px-3 py-2 text-sm text-[var(--color-warning)]"
              >
                <p>
                  This contract is already tracked
                  {validation.label ? ` as “${validation.label}”` : ""}.
                </p>
                <Link
                  href={`/contracts/${trimmedId}`}
                  className="mt-2 inline-block underline"
                >
                  Open the existing contract
                </Link>
              </div>
            )}
          </div>
        )}

        {/* ---- Step 3: configure ---- */}
        {step === 2 && (
          <div className="space-y-4" data-testid="wizard-step-configure">
            <h2 className="text-lg font-semibold">Label and monitoring</h2>

            <div>
              <label htmlFor="wizard-label" className="block text-sm font-medium">
                Label <span className="text-[var(--color-text-secondary)]">(optional)</span>
              </label>
              <input
                id="wizard-label"
                data-testid="wizard-label"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder="My Token Contract"
                aria-invalid={labelError ? true : undefined}
                aria-describedby={labelError ? "wizard-label-error" : undefined}
                className="mt-1 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-page)] px-3 py-2 text-sm focus:border-[var(--color-accent)] focus:outline-none"
              />
              {labelError && (
                <p
                  id="wizard-label-error"
                  data-testid="wizard-label-error"
                  className="mt-1 text-xs text-[var(--color-danger)]"
                >
                  {labelError}
                </p>
              )}
            </div>

            <div className="rounded-md border border-[var(--color-border)] p-3">
              <label className="flex items-start gap-3 text-sm">
                <input
                  type="checkbox"
                  data-testid="wizard-watchdog"
                  checked={watchdog}
                  onChange={(e) => setWatchdog(e.target.checked)}
                  className="mt-1"
                />
                <span>
                  <span className="font-medium">Enroll in Watchdog monitoring</span>
                  <span className="mt-1 block text-xs text-[var(--color-text-secondary)]">
                    Watchdog enrollment is recorded by the on-chain
                    sorolens-watchdog contract. Checking this finishes on the
                    Watchdog page after the contract is created; leave it
                    unchecked to go straight to the contract.
                  </span>
                </span>
              </label>
            </div>

            {error && (
              <p
                role="alert"
                data-testid="wizard-create-error"
                className="rounded-md border border-[var(--color-danger)]/40 bg-[var(--color-danger)]/10 px-3 py-2 text-sm text-[var(--color-danger)]"
              >
                {error}
              </p>
            )}
          </div>
        )}

        {/* ---- Controls ---- */}
        <div className="mt-6 flex items-center justify-between gap-3">
          <button
            type="button"
            data-testid="wizard-back"
            onClick={handleBack}
            disabled={step === 0 || submitting}
            className="rounded-md border border-[var(--color-border)] px-4 py-2 text-sm disabled:opacity-40"
          >
            Back
          </button>

          {step < 2 ? (
            <button
              type="button"
              data-testid="wizard-next"
              onClick={handleNext}
              disabled={!nextEnabled || validation.status === "checking"}
              className="rounded-md bg-[var(--color-accent)] px-4 py-2 text-sm font-medium text-[var(--color-bg-page)] disabled:opacity-40"
            >
              Next
            </button>
          ) : (
            <button
              type="button"
              data-testid="wizard-create"
              onClick={handleCreate}
              disabled={submitting || Boolean(labelError)}
              className="rounded-md bg-[var(--color-accent)] px-4 py-2 text-sm font-medium text-[var(--color-bg-page)] disabled:opacity-40"
            >
              {submitting ? "Creating…" : "Create contract"}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
