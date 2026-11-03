"use client";

import { LifecyclePolicyEditorPage } from "@/components/lifecycle/LifecyclePolicyEditorPage";

/** Its own route, so a policy named "new" stays reachable at /lifecycle-policies/new. */
export default function NewLifecyclePolicyPage() {
  return <LifecyclePolicyEditorPage creating />;
}
