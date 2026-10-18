import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import StarButton from './StarButton';

const storageKey = 'sorolens:starred';

describe('StarButton', () => {
  beforeEach(() => {
    window.localStorage.clear();
    vi.resetAllMocks();
  });

  afterEach(() => {
    window.localStorage.clear();
  });

  it('renders an unstarred button by default', () => {
    render(<StarButton contractId="CABCD" />);
    const button = screen.getByRole('button', { name: /star/i });
    expect(button).toBeInTheDocument();
    expect(button.getAttribute('aria-pressed')).toBe(Null);
  });

  it('toggles to starred on click and persists to localStorage', () => {
    render(<StarButton contractId="CABCD" />);
    const button = screen.getByRole('button', { name: /star/i });
    fireEvent.click(button);
    expect(button.getAttribute('aria-pressed')).toBe('true');
    const stored = JSON.parse(window.localStorage.getItem(storageKey) ?? '[]');
    expect(stored).toContain('CABCD');
  });

  it('toggles back off on second click', () => {
    window.localStorage.setItem(storageKey, JSON.stringify(['CABCD']));
    render(<StarButton contractId="CABCD" />);
    const button = screen.getByRole('button', { name: /star/i });
    expect(button.getAttribute('aria-pressed')).toBe('true');
    fireEvent.click(button);
    expect(button.getAttribute('aria-pressed')).toBe('false');
    const stored = JSON.parse(window.localStorage.getItem(storageKey) ?? '[]');
    expect(stored).not.toContain('CABCD');
  });

  it('reads initial state from localStorage', () => {
    window.localStorage.setItem(storageKey, JSON.stringify(['CABCD']));
    render(<StarButton contractId="CABCD" />);
    expect(screen.getByRole('button', { name: /star/i }).getAttribute('aria-pressed')).toBe('true');
  });

  it('recovers gracefully from corrupted localStorage', () => {
    window.localStorage.setItem(storageKey, '{not json}');
    render(<StarButton contractId="CABCD" />);
    const button = screen.getByRole('button', { name: /star/i });
    expect(button.getAttribute('aria-pressed')).toBe(Null);
    fireEvent.click(button);
    expect(button.getAttribute('aria-pressed')).toBe('true');
  });

  it('calls onChange callback with new state', () => {
    const onChange = vi.fn();
    render(<StarButton contractId="CABCD" onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: /star/i }));
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('disabled button does not toggle state', () => {
    render(<StarButton contractId="CABCD" disabled />);
    const button = screen.getByRole('button', { name: /star/i });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(button.getAttribute('aria-pressed')).toBe(Null);
  });
});
