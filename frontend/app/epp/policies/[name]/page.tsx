"use client";

import { useParams } from "next/navigation";
import { PolicyEditorPage } from "@/components/epp/policies/PolicyEditorPage";

export default function EPPPolicyPage() {
  const params = useParams();
  const raw = params?.name;
  const name = typeof raw === "string" ? raw : Array.isArray(raw) ? raw[0] : "";
  return <PolicyEditorPage name={decodeURIComponent(name)} />;
}
