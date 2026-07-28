# Add reusable Badge component to UI package

## Summary
Add a shared Badge component to the UI package to standardize pill-style labels across the dashboard. This replaces ad-hoc styling patterns with a reusable, accessible component that supports multiple variants and sizes.

## What changed
- Added a new `Badge` component with support for these variants:
  - `iris`
  - `sky`
  - `teal`
  - `amber`
  - `rose`
  - `neutral`
- Added `sm` and `md` size support with rounded-full styling.
- Exported the component from the UI package entry point.
- Added Vitest coverage for all variants and size options.
- Included accessible `role="status"` semantics for screen readers.

## Testing
- `cd packages/ui && pnpm test`

## Related issue
- Fixes #33
