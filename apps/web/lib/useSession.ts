"use client";

import { useEffect, useState } from "react";
import { ensureSession, getUserId } from "./user";

/**
 * Returns the verified workspace user ID, minting a session on first use.
 * The first render is always empty so server and client markup agree; effects
 * run once the session exists.
 */
export function useSessionUserId(): string {
  const [userId, setUserId] = useState("");

  useEffect(() => {
    let active = true;
    const cached = getUserId();
    if (cached) setUserId(cached);
    ensureSession()
      .then((session) => {
        if (active) setUserId(session.user_id);
      })
      .catch(() => {
        // Leave the user anonymous; identity-scoped reads will be empty.
      });
    return () => {
      active = false;
    };
  }, []);

  return userId;
}
