"use client";

import { FamilyEditorPage } from "@/components/epp/families/FamilyEditorPage";

/** Its own route, so a family named "new" stays reachable at /epp/families/new. */
export default function NewRegistrarFamilyPage() {
  return <FamilyEditorPage creating />;
}
