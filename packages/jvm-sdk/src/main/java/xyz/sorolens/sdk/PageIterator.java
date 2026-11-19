package xyz.sorolens.sdk;

import java.util.Collections;
import java.util.Iterator;
import java.util.List;
import java.util.NoSuchElementException;
import java.util.function.Function;

/**
 * Iterates every item of a cursor-paginated endpoint.
 *
 * <p>The fetcher is called with the current cursor ({@code null} for the first
 * page) and returns the next {@link Page}. Iteration stops when the API reports
 * no next cursor. Pages are fetched lazily, so a partial walk costs only the
 * pages it read.
 */
public final class PageIterator<T> implements Iterator<T>, Iterable<T> {

    private final Function<String, Page<T>> fetcher;
    private String cursor;
    private Iterator<T> current = Collections.emptyIterator();
    private boolean done;

    public PageIterator(Function<String, Page<T>> fetcher) {
        this(fetcher, null);
    }

    public PageIterator(Function<String, Page<T>> fetcher, String startCursor) {
        this.fetcher = fetcher;
        this.cursor = startCursor;
    }

    @Override
    public Iterator<T> iterator() {
        return this;
    }

    @Override
    public boolean hasNext() {
        while (!current.hasNext() && !done) {
            Page<T> page = fetcher.apply(cursor);
            List<T> items = page.items();
            current = items.iterator();
            if (page.hasMore()) {
                cursor = page.nextCursor();
            } else {
                done = true;
            }
            if (items.isEmpty() && !done) {
                continue;
            }
            if (items.isEmpty()) {
                return false;
            }
        }
        return current.hasNext();
    }

    @Override
    public T next() {
        if (!hasNext()) {
            throw new NoSuchElementException();
        }
        return current.next();
    }
}
