import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import SnapshotPanel from './SnapshotPanel';

/**
 * Unit tests for SnapshotPanel.
 *
 * Edge cases covered:
 *  - Rendering the default empty state before any snapshot is created.
 *  - Creating a snapshot successfully (invokes the callback and shows confirmation).
 *  - Failure path when the create callback rejects (error message is surfaced).
 *  - Disabled button while a snapshot is in flight (prevents double submit).
 *  - Rendering a list of existing snapshots and restoring one.
 *  - Restore failure path.
 */

describe('SnapshotPanel', () => {
  const createSnapshot = vi.fn();
  const restoreSnapshot = vi.fn();

  beforeEach(() => {
    createSnapshot.mockReset();
    restoreSnapshot.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the empty state when there are no snapshots', () => {
    render(
      <SnapshotPanel
        snapshots={[]}
        onCreateSnapshot={createSnapshot}
        onRestoreSnapshot={restoreSnapshot}
      />,
    );

    expect(screen.getByText(/no snapshots/i)).toBeInTheDocument();
  });

  it('creates a snapshot and shows a confirmation', async () => {
    createSnapshot.mockResolved({ id: 'snap-1', createdAt: '2024-01-01T00:00:00Z' });
    const user = userEvent.setup();

    render(
      <SnapshotPanel
        snapshots={[]}
        onCreateSnapshot={createSnapshot}
        onRestoreSnapshot={restoreSnapshot}
      />,
    );

    await user.click(screen.getByRole('button', { name: /create snapshot/i }));

    expect(createSnapshot).toHaveBeenCalledTimes(1);
    await waitFor(() =>
      expect(screen.getByText(/snapshot created/i)).toBeInTheDocument(),
    );
  });

  it('surfaces an error message when creating a snapshot fails', async () => {
    createSnapshot.mockRejected(new Error('disk full'));
    const user = userEvent.setup();

    render(
      <SnapshotPanel
        snapshots={[]}
        onCreateSnapshot={createSnapshot}
        onRestoreSnapshot={restoreSnapshot}
      />,
    );

    await user.click(screen.getByRole('button', { name: /create snapshot/i }));

    await waitFor(() =>
      expect(screen.getByText(/disk full/i)).toBeInTheDocument(),
    );
  });

  it('disables the create button while a snapshot is in flight', async () => {
    let resolve: () => void = () => {};
    createSnapshot.mockImplementation(
      () =>
        new Promise((res) => {
          resolve = () => res({ id: 'snap-2', createdAt: '2024-01-01T00:00:00Z' });
        }),
    );
    const user = userEvent.setup();

    render(
      <SnapshotPanel
        snapshots={[]}
        onCreateSnapshot={createSnapshot}
        onRestoreSnapshot={restoreSnapshot}
      />,
    );

    const button = screen.getByRole('button', { name: /create snapshot/i });
    await user.click(button);

    expect(button).toBeDisabled();
    expect(createSnapshot).toHaveBeenCalledTimes(1);

    resolve();
    await waitFor(() => expect(button).not.toBeDisabled());
  });

  it('renders existing snapshots and restores the selected one', async () => {
    restoreSnapshot.mockResolved(undefined);
    const user = userEvent.setup();
    const snapshots = [
      { id: 'snap-1', createdAt: '2024-01-01T00:00:00Z' },
      { id: 'snap-2', createdAt: '2024-02-01T00:00:00Z' },
    ];

    render(
      <SnapshotPanel
        snapshots={snapshots}
        onCreateSnapshot={createSnapshot}
        onRestoreSnapshot={restoreSnapshot}
      />,
    );

    expect(screen.getByText('snap-1')).toBeInTheDocument();
    expect(screen.getByText('snap-2')).toBeInTheDocument();

    const restoreButtons = screen.getAllByRole('button', { name: /restore/i });
    expect(restoreButtons).toHaveLength(2);
    await user.click(restoreButtons[1]);

    expect(restoreSnapshot).toHaveBeenCalledWith('snap-2');
  });

  it('surfaces an error message when restoring a snapshot fails', async () => {
    restoreSnapshot.mockRejected(new Error('snapshot corrupted'));
    const user = userEvent.setup();
    const snapshots = [{ id: 'snap-1', createdAt: '2024-01-01T00:00:00Z' }];

    render(
      <SnapshotPanel
        snapshots={snapshots}
        onCreateSnapshot={createSnapshot}
        onRestoreSnapshot={restoreSnapshot}
      />,
    );

    await user.click(screen.getByRole('button', { name: /restore/i }));

    await waitFor(() =>
      expect(screen.getByText(/snapshot corrupted/i)).toBeInTheDocument(),
    );
  });
});
