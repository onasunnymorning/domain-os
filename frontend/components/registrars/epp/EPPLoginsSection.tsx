"use client";

import { Fragment, useState } from "react";
import { toast } from "sonner";
import { ChevronDown, ChevronRight } from "lucide-react";
import { format } from "date-fns";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EPPSectionHeader } from "@/components/epp/EPPSectionHeader";
import { CredentialStateBadge } from "@/components/epp/CredentialStateBadge";
import type { RetireTarget } from "@/components/epp/RetireCredentialDialog";
import { useLiftEPPLockouts, useLiftEPPSuspension, useUpdateEPPLogin, eppErrorMessage } from "@/lib/hooks/useEPP";
import type { EPPLogin, EPPLoginLockout, EPPLoginSuspension } from "@/lib/types/epp";
import { AddLoginDialog } from "./AddLoginDialog";
import { AddPasswordDialog } from "./AddPasswordDialog";
import { EditLoginDialog } from "./EditLoginDialog";
import { LastUsed } from "./LastUsed";

const MAX_LOGINS = 10;
const MAX_USABLE_PASSWORDS = 2;

export function RoleBadge({ role }: { role: EPPLogin["role"] }) {
  return role === "readonly" ? (
    <Badge variant="outline">Read-only</Badge>
  ) : (
    <Badge variant="secondary">Transform</Badge>
  );
}

export function EPPLoginsSection({
  clid,
  logins,
  lockouts,
  suspensions,
  onRetire,
}: {
  clid: string;
  logins: EPPLogin[];
  lockouts: EPPLoginLockout[];
  suspensions: EPPLoginSuspension[];
  onRetire: (t: RetireTarget) => void;
}) {
  const update = useUpdateEPPLogin(clid);
  const liftLockouts = useLiftEPPLockouts();
  const liftSuspension = useLiftEPPSuspension();
  const [addOpen, setAddOpen] = useState(false);
  const [passwordFor, setPasswordFor] = useState<string | null>(null);
  const [editing, setEditing] = useState<EPPLogin | null>(null);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  const toggleEnabled = async (login: EPPLogin, enabled: boolean) => {
    try {
      await update.mutateAsync({ login: login.login_id, body: { enabled } });
      toast.success(
        enabled
          ? `${login.login_id} can log in again.`
          : `${login.login_id} can no longer log in. Sessions already open run until they end.`,
      );
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not change the login"));
    }
  };

  const lift = async (kind: "lockout" | "suspension", login: string) => {
    try {
      if (kind === "lockout") {
        await liftLockouts.mutateAsync({ clid, login });
        toast.success(`${login} is no longer locked out.`);
      } else {
        await liftSuspension.mutateAsync({ clid, login });
        toast.success(`${login} is no longer suspended. Its strikes are cleared.`);
      }
    } catch (err) {
      toast.error(eppErrorMessage(err, `Could not lift the ${kind}`));
    }
  };

  return (
    <Card id="epp-logins">
      <EPPSectionHeader
        title="Logins"
        description="The client IDs this registrar's systems log in with. One login per system or purpose; each has its own passwords, role and session cap. A login holds up to two usable passwords so one can be rolled without an outage."
        count={`${logins.length} of ${MAX_LOGINS}`}
        action={
          <Button size="sm" onClick={() => setAddOpen(true)} disabled={logins.length >= MAX_LOGINS}>
            Add login
          </Button>
        }
      />
      <CardContent>
        {logins.length === 0 ? (
          <div className="rounded-md border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200">
            No login yet. Add one, then give it a password.
          </div>
        ) : (
          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-8" />
                  <TableHead>Login</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Enabled</TableHead>
                  <TableHead>Session cap</TableHead>
                  <TableHead>Passwords</TableHead>
                  <TableHead className="w-[160px]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {logins.map((l) => {
                  const usable = l.passwords.filter((p) => p.state !== "retired");
                  const isOpen = expanded[l.login_id] ?? false;
                  const myLockouts = lockouts.filter((k) => k.login === l.login_id);
                  const mySuspension = suspensions.find((s) => s.login === l.login_id);
                  const problem = myLockouts.length > 0 || !!mySuspension;
                  return (
                    <Fragment key={l.login_id}>
                      <TableRow className={!l.enabled ? "text-muted-foreground" : undefined}>
                        <TableCell>
                          <button
                            type="button"
                            aria-label={isOpen ? `Hide passwords of ${l.login_id}` : `Show passwords of ${l.login_id}`}
                            aria-expanded={isOpen}
                            onClick={() => setExpanded((e) => ({ ...e, [l.login_id]: !isOpen }))}
                            className="rounded p-1 hover:bg-accent"
                          >
                            {isOpen ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                          </button>
                        </TableCell>
                        <TableCell className="font-mono font-medium">{l.login_id}</TableCell>
                        <TableCell>
                          <RoleBadge role={l.role} />
                        </TableCell>
                        <TableCell>
                          <Switch
                            checked={l.enabled}
                            aria-label={`${l.login_id} enabled`}
                            onCheckedChange={(v) => toggleEnabled(l, v)}
                            disabled={update.isPending}
                          />
                        </TableCell>
                        <TableCell>
                          {l.session_cap === 0 ? <span className="text-muted-foreground">policy default</span> : l.session_cap}
                        </TableCell>
                        <TableCell>
                          {usable.length === 0 ? (
                            <span className="text-amber-700 dark:text-amber-400">none usable</span>
                          ) : (
                            `${usable.length} usable`
                          )}
                        </TableCell>
                        <TableCell className="text-right space-x-1 whitespace-nowrap">
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => setPasswordFor(l.login_id)}
                            disabled={usable.length >= MAX_USABLE_PASSWORDS}
                            title={usable.length >= MAX_USABLE_PASSWORDS ? "Retire a password first" : undefined}
                          >
                            Add password
                          </Button>
                          <Button size="sm" variant="ghost" onClick={() => setEditing(l)}>
                            Edit
                          </Button>
                        </TableCell>
                      </TableRow>
                      {problem && (
                        <TableRow className="bg-amber-50 dark:bg-amber-950/30 hover:bg-amber-50">
                          <TableCell />
                          <TableCell colSpan={6}>
                            <div className="space-y-1 text-sm">
                              {myLockouts.map((k) => (
                                <div key={k.client_ip} className="flex items-center justify-between gap-2">
                                  <span>
                                    Locked out from <span className="font-mono">{k.client_ip}</span> until{" "}
                                    {format(new Date(k.ends_at), "MMM d, HH:mm")} after too many wrong passwords.
                                  </span>
                                  <Button size="sm" variant="outline" onClick={() => lift("lockout", l.login_id)} disabled={liftLockouts.isPending}>
                                    Lift lockout
                                  </Button>
                                </div>
                              ))}
                              {mySuspension && (
                                <div className="flex items-center justify-between gap-2">
                                  <span>
                                    Suspended until {format(new Date(mySuspension.ends_at), "MMM d, HH:mm")} for creating registered
                                    names{mySuspension.scope === "family" ? ` (counted for family ${mySuspension.family})` : ""}.
                                    Lifting also clears the strikes.
                                  </span>
                                  <Button size="sm" variant="outline" onClick={() => lift("suspension", l.login_id)} disabled={liftSuspension.isPending}>
                                    Lift suspension
                                  </Button>
                                </div>
                              )}
                            </div>
                          </TableCell>
                        </TableRow>
                      )}
                      {isOpen && (
                        <TableRow className="bg-muted/30 hover:bg-muted/30">
                          <TableCell />
                          <TableCell colSpan={6} className="py-3">
                            {l.passwords.length === 0 ? (
                              <div className="text-sm text-muted-foreground">No password yet.</div>
                            ) : (
                              <Table>
                                <TableHeader>
                                  <TableRow>
                                    <TableHead>Password added</TableHead>
                                    <TableHead>Last used</TableHead>
                                    <TableHead>State</TableHead>
                                    <TableHead className="w-[100px]" />
                                  </TableRow>
                                </TableHeader>
                                <TableBody>
                                  {l.passwords.map((p) => (
                                    <TableRow key={p.id} className={p.state === "retired" ? "text-muted-foreground" : undefined}>
                                      <TableCell>
                                        {format(new Date(p.created_at), "MMM d, yyyy HH:mm")}
                                        {p.created_by && <span className="text-muted-foreground"> by {p.created_by}</span>}
                                      </TableCell>
                                      <TableCell>
                                        <LastUsed at={p.last_used_at} from={p.last_used_from} />
                                      </TableCell>
                                      <TableCell>
                                        <CredentialStateBadge credential={p} />
                                        {p.retire_kind === "replaced" && (
                                          <span className="ml-2 text-xs text-muted-foreground">replaced by the registrar</span>
                                        )}
                                      </TableCell>
                                      <TableCell className="text-right">
                                        {p.retire_kind !== "compromise" && (
                                          <Button
                                            size="sm"
                                            variant="ghost"
                                            onClick={() =>
                                              onRetire({
                                                kind: "password",
                                                id: p.id,
                                                login: l.login_id,
                                                label: `the password of ${l.login_id} added ${format(new Date(p.created_at), "MMM d")}`,
                                                alreadyRetired: p.state !== "active",
                                              })
                                            }
                                          >
                                            {p.state === "active" ? "Retire" : "Revoke"}
                                          </Button>
                                        )}
                                      </TableCell>
                                    </TableRow>
                                  ))}
                                </TableBody>
                              </Table>
                            )}
                          </TableCell>
                        </TableRow>
                      )}
                    </Fragment>
                  );
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      <AddLoginDialog
        clid={clid}
        open={addOpen}
        onOpenChange={setAddOpen}
        onCreated={(loginId) => {
          setExpanded((e) => ({ ...e, [loginId]: true }));
          setPasswordFor(loginId);
        }}
      />
      <AddPasswordDialog clid={clid} login={passwordFor} onOpenChange={(v) => !v && setPasswordFor(null)} />
      <EditLoginDialog clid={clid} login={editing} onOpenChange={(v) => !v && setEditing(null)} />
    </Card>
  );
}
