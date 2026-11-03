"use client";

import { LifecyclePolicyEditorPage } from "@/components/lifecycle/LifecyclePolicyEditorPage";

/** Under "-", which no policy name may start with, so no policy's own URL (/lifecycle-policies/{name}) can collide with it. */
export default function NewLifecyclePolicyPage() {
  return <LifecyclePolicyEditorPage creating />;
}
