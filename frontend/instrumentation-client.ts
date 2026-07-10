import posthog from "posthog-js";
import { getPostHogToken } from "@/lib/env";

// NODE_ENV stays on process.env — it is a build-time constant, not runtime config.
const token = getPostHogToken();

// The token now resolves at runtime, so it can legitimately be absent
// (e.g. local dev without analytics). Initialising with an empty token
// makes posthog-js throw on every capture.
if (token) {
  posthog.init(token, {
    api_host: "/ingest",
    ui_host: "https://us.posthog.com",
    defaults: "2026-01-30",
    capture_exceptions: true,
    debug: process.env.NODE_ENV === "development",
  });
}
