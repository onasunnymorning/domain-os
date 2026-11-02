"use client";

import { useState } from "react";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useDebounce } from "@/lib/hooks/useDebounce";
import { useRegistrar, useRegistrars } from "@/lib/hooks/useRegistrars";
import { useEPPPolicies } from "@/lib/hooks/useEPP";
import { POLICY_NAME_PATTERN } from "@/lib/constants/eppPolicyDefaults";
import type { RegistrarListItem } from "@/lib/types/registrar";

export interface FamilyFormValues {
  name: string;
  description: string;
  policy: string;
  members: string[];
}

export function FamilyForm({
  mode,
  initial,
  saving = false,
  /** The server's message about a member, shown beside that member (e.g. already in another family). */
  memberError,
  onSave,
  onDelete,
}: {
  mode: "create" | "edit";
  initial: FamilyFormValues;
  saving?: boolean;
  memberError?: { clid: string; message: string } | null;
  onSave: (v: FamilyFormValues) => Promise<void>;
  onDelete?: () => void;
}) {
  const [v, setV] = useState<FamilyFormValues>(initial);
  const [search, setSearch] = useState("");
  const debounced = useDebounce(search, 300);
  const policies = useEPPPolicies();
  const term = debounced.trim();
  // A substring search of the first 25, plus an exact lookup so a complete ID
  // is always offered even when many IDs contain it.
  const candidates = useRegistrars(term ? { clid_like: term, pagesize: 25 } : undefined);
  const exact = useRegistrar(term, !!term);
  const nameValid = POLICY_NAME_PATTERN.test(v.name);
  const found = (() => {
    const list = candidates.data?.Data ?? [];
    if (exact.data && !list.some((r) => r.ClID === exact.data!.ClID)) {
      return [{ ClID: exact.data.ClID, Name: exact.data.Name } as RegistrarListItem, ...list];
    }
    return list;
  })();

  const add = (clid: string) => {
    if (!v.members.includes(clid)) setV((s) => ({ ...s, members: [...s.members, clid] }));
    setSearch("");
  };
  const remove = (clid: string) => setV((s) => ({ ...s, members: s.members.filter((m) => m !== clid) }));

  return (
    <form
      className="space-y-6"
      onSubmit={(e) => {
        e.preventDefault();
        onSave(v);
      }}
    >
      <Card>
        <CardHeader>
          <CardTitle>Name and policy</CardTitle>
          <CardDescription className="max-w-prose">
            The family&apos;s policy supplies the shared limits: its sessions per family, family rates and family
            failed-create budget. Each member keeps its own policy for its own limits.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="family-name">Name</Label>
            <Input
              id="family-name"
              value={v.name}
              onChange={(e) => setV((s) => ({ ...s, name: e.target.value }))}
              disabled={mode === "edit"}
              className="max-w-xs font-mono"
              placeholder="acme-group"
            />
            <p className="text-xs text-muted-foreground">
              1 to 32 lower-case letters, digits and hyphens, starting with a letter. Cannot change later.
            </p>
            {v.name && !nameValid && <p className="text-xs text-destructive">That is not a valid name.</p>}
          </div>
          <div className="space-y-2">
            <Label htmlFor="family-description">Description</Label>
            <Input
              id="family-description"
              value={v.description}
              onChange={(e) => setV((s) => ({ ...s, description: e.target.value }))}
              placeholder="Acme Corp's three accreditations"
            />
          </div>
          <div className="space-y-2">
            <Label>Policy</Label>
            <Select value={v.policy || "default"} onValueChange={(p) => setV((s) => ({ ...s, policy: p === "default" ? "" : p }))}>
              <SelectTrigger className="max-w-sm" aria-label="Family policy">
                <SelectValue placeholder="default" />
              </SelectTrigger>
              <SelectContent>
                {(policies.data ?? []).map((p) => (
                  <SelectItem key={p.name} value={p.name}>
                    <span className="font-mono">{p.name}</span>
                    {p.description && <span className="text-muted-foreground"> · {p.description}</span>}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">Only the policy&apos;s family limits apply here. &quot;default&quot; uses the default policy&apos;s.</p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Members</CardTitle>
          <CardDescription className="max-w-prose">
            The registrars counted together. A registrar can be in one family only; adding one that is already in
            another is refused, and the message says which.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {v.members.length === 0 ? (
            <p className="text-sm text-muted-foreground">No members yet.</p>
          ) : (
            <ul className="space-y-1">
              {v.members.map((m) => (
                <li key={m} className="flex items-center gap-2 text-sm">
                  <span className="font-mono">{m}</span>
                  {memberError?.clid === m && <span className="text-xs text-destructive">{memberError.message}</span>}
                  <button type="button" aria-label={`Remove ${m}`} onClick={() => remove(m)} className="rounded p-0.5 text-muted-foreground hover:text-foreground">
                    <X className="h-3.5 w-3.5" />
                  </button>
                </li>
              ))}
            </ul>
          )}
          <div className="space-y-2">
            <Label htmlFor="family-member-search">Add a registrar</Label>
            <Input
              id="family-member-search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search by client ID…"
              className="max-w-sm font-mono"
            />
            {term && (
              <div className="max-w-sm rounded-md border">
                {candidates.isLoading ? (
                  <div className="p-3 text-sm text-muted-foreground">Searching…</div>
                ) : candidates.error ? (
                  <div className="flex items-center justify-between gap-2 p-3 text-sm text-red-700 dark:text-red-400">
                    <span>Could not search registrars: {(candidates.error as { response?: { data?: { error?: string } } })?.response?.data?.error || "the request failed"}</span>
                    <Button type="button" size="sm" variant="outline" onClick={() => candidates.refetch()}>
                      Retry
                    </Button>
                  </div>
                ) : found.length === 0 ? (
                  <div className="p-3 text-sm text-muted-foreground">No registrar matches.</div>
                ) : (
                  <ul>
                    {found.map((r: RegistrarListItem) => (
                      <li key={r.ClID}>
                        <button
                          type="button"
                          onClick={() => add(r.ClID)}
                          disabled={v.members.includes(r.ClID)}
                          className="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-accent disabled:opacity-50"
                        >
                          <span className="font-mono">{r.ClID}</span>
                          <span className="text-muted-foreground">{r.Name}</span>
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center justify-between">
        <div>
          {mode === "edit" && onDelete && (
            <Button type="button" variant="destructive" onClick={onDelete}>
              Delete family
            </Button>
          )}
        </div>
        <Button type="submit" disabled={saving || !nameValid}>
          {saving ? "Saving…" : mode === "create" ? "Create family" : "Save"}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        Takes effect within about 30 seconds. A registrar that joins counts against the family from then on; its open
        sessions stay open.
      </p>
    </form>
  );
}
