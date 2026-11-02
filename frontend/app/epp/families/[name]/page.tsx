"use client";

import { useParams } from "next/navigation";
import { FamilyEditorPage } from "@/components/epp/families/FamilyEditorPage";

export default function RegistrarFamilyPage() {
  const params = useParams();
  const raw = params?.name;
  const name = typeof raw === "string" ? raw : Array.isArray(raw) ? raw[0] : "";
  return <FamilyEditorPage name={decodeURIComponent(name)} />;
}
