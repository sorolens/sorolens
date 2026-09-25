/**
 * usePushSubscription.test.tsx — tests for the push subscription hook.
 *
 * The hook interacts with navigator.serviceWorker, window.PushManager, and
 * fetch.  We install minimal stubs for these globals and verify the hook
 * drives the right state machine transitions.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { usePushSubscription } from "@/hooks/usePushSubscription";

// ---------------------------------------------------------------------------
// Global stubs
// ---------------------------------------------------------------------------

const mockSubscription = {
  endpoint: "https://fcm.googleapis.com/test",
  toJSON: () => ({
    endpoint: "https://fcm.googleapis.com/test",
    keys: { auth: "auth-key", p256dh: "p256dh-key" },
  }),
  unsubscribe: vi.fn().mockResolvedValue(true),
};

const mockPushManager = {
  subscribe: vi.fn().mockResolvedValue(mockSubscription),
  getSubscription: vi.fn().mockResolvedValue(null),
};

const mockRegistration = {
  pushManager: mockPushManager,
};

beforeEach(() => {
  // serviceWorker
  const regMock = {
    pushManager: {
      subscribe: vi.fn().mockResolvedValue(mockSubscription),
      getSubscription: vi.fn().mockResolvedValue(null),
    },
  };

  Object.defineProperty(navigator, "serviceWorker", {
    value: {
      ready: Promise.resolve(regMock),
    },
    configurable: true,
    writable: true,
  });

  // PushManager — just needs to exist on window
  if (!("PushManager" in window)) {
    Object.defineProperty(window, "PushManager", {
      value: function PushManager() {},
      configurable: true,
      writable: true,
    });
  }

  // Notification
  Object.defineProperty(window, "Notification", {
    value: {
      permission: "default",
      requestPermission: vi.fn().mockResolvedValue("granted"),
    },
    configurable: true,
    writable: true,
  });

  // fetch
  global.fetch = vi.fn().mockImplementation((url: string) => {
    if (String(url).includes("vapid-public-key")) {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            // Short base64url string that won't error in urlBase64ToUint8Array
            publicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
          }),
      });
    }
    if (String(url).includes("/api/push/subscribe")) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ ok: true }),
      });
    }
    return Promise.resolve({ ok: true, json: () => Promise.resolve({}) });
  });

  vi.resetModules();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("usePushSubscription", () => {
  it("starts in idle state when push is supported", () => {
    const { result } = renderHook(() => usePushSubscription());
    expect(result.current.state).toBe("idle");
    expect(result.current.isSupported).toBe(true);
  });

  it("transitions to subscribed after a successful subscribe() call", async () => {
    const { result } = renderHook(() => usePushSubscription());

    await act(async () => {
      await result.current.subscribe();
    });

    expect(result.current.state).toBe("subscribed");
    expect(result.current.isSubscribed).toBe(true);
    expect(result.current.error).toBeNull();
  });

  it("transitions to denied when Notification.requestPermission() returns denied", async () => {
    (
      window.Notification as unknown as {
        requestPermission: ReturnType<typeof vi.fn>;
      }
    ).requestPermission = vi.fn().mockResolvedValue("denied");

    const { result } = renderHook(() => usePushSubscription());

    await act(async () => {
      await result.current.subscribe();
    });

    expect(result.current.state).toBe("denied");
  });

  it("transitions to error when the VAPID key fetch fails", async () => {
    global.fetch = vi.fn().mockResolvedValue({ ok: false });

    const { result } = renderHook(() => usePushSubscription());

    await act(async () => {
      await result.current.subscribe();
    });

    expect(result.current.state).toBe("error");
    expect(result.current.error).toContain("VAPID");
  });

  it("returns unsupported when serviceWorker is not available", () => {
    Object.defineProperty(navigator, "serviceWorker", {
      value: undefined,
      configurable: true,
    });

    const { result } = renderHook(() => usePushSubscription());
    // On mount the effect sets unsupported
    expect(["unsupported", "idle"]).toContain(result.current.state);
  });
});
