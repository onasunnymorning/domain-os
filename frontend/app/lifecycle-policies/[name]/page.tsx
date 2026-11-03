"use client";

import { useParams } from "next/navigation";
import { LifecyclePolicyEditorPage } from "@/components/lifecycle/LifecyclePolicyEditorPage";

export default function LifecyclePolicyPage() {
  const params = useParams();
  const raw = params?.name;
  const name = typeof raw === "string" ? raw : Array.isArray(raw) ? raw[0] : "";
  return <LifecyclePolicyEditorPage name={decodeURIComponent(name)} />;
}
