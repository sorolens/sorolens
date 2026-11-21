"""Shared cursor-pagination rules for the ``iter`` helpers.

Every list endpoint returns a page plus an opaque ``next_cursor``; an empty
cursor marks the last page. The helpers here keep the sync and async
iterators honest about two things a hand-rolled loop tends to get wrong:

* the page size is kept inside the server's limit (``1..=200``), and
* a cursor that does not advance stops the walk with
  :class:`~sorolens.exceptions.PaginationError` instead of looping forever
  or silently re-reading the same rows.
"""

from __future__ import annotations

from .exceptions import PaginationError

#: Largest ``limit`` the list endpoints accept (``docs/openapi.yaml``).
MAX_PAGE_SIZE = 200


def page_limit(limit: int | None) -> int | None:
    """Return the ``limit`` to send, clamped to the server's maximum.

    ``None`` keeps the server default. A value below 1 is rejected before any
    request is made, since the server would answer it with a 422.
    """
    if limit is None:
        return None
    if isinstance(limit, bool) or not isinstance(limit, int):
        raise TypeError(f"limit must be an int, got {type(limit).__name__}")
    if limit < 1:
        raise ValueError(f"limit must be at least 1, got {limit}")
    return min(limit, MAX_PAGE_SIZE)


class CursorTracker:
    """Remembers the cursors already requested during one walk."""

    def __init__(self) -> None:
        self._seen: set[str] = set()

    def advance(self, next_cursor: str | None) -> str | None:
        """Return the cursor for the next page, or ``None`` on the last page.

        Raises :class:`PaginationError` when the server hands back a cursor
        this walk has already requested. The items of the current page have
        been yielded by then, so callers keep everything received so far.
        """
        if not next_cursor:
            return None
        if next_cursor in self._seen:
            raise PaginationError(
                f"the API returned cursor {next_cursor!r} twice; stopping to avoid a loop"
            )
        self._seen.add(next_cursor)
        return next_cursor
