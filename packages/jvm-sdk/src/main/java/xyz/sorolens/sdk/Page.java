package xyz.sorolens.sdk;

import java.util.List;

/**
 * One cursor page.
 *
 * @param items      the page contents, in API order
 * @param nextCursor the cursor for the next page, or {@code null}/blank at the end
 */
public record Page<T>(List<T> items, String nextCursor) {

    public Page {
        items = List.copyOf(items);
    }

    public boolean hasMore() {
        return nextCursor != null && !nextCursor.isBlank();
    }
}
