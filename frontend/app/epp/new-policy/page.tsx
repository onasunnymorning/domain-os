"use client";

import { PolicyEditorPage } from "@/components/epp/policies/PolicyEditorPage";

/** Its own route, so a policy named "new" stays reachable at /epp/policies/new. */
export default function NewEPPPolicyPage() {
  return <PolicyEditorPage creating />;
}
