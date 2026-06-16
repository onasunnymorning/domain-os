"use client";

import { useState, useMemo } from "react";
import { DomainDetail } from "@/lib/types/domain";
import { useSetDropCatch, useUnsetDropCatch, useUpdateDomain } from "@/lib/hooks/useDomains";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "sonner";
import { Settings2, CalendarIcon, RefreshCcw } from "lucide-react";
import { Calendar } from "@/components/ui/calendar";
import { format } from "date-fns";
import { cn } from "@/lib/utils";

// Helper to convert an ISO UTC string into a "local" Date object that visually shows the exact same Y/M/D/H/M 
function hydrateUTCDate(isoString: string | null | undefined): Date | undefined {
  if (!isoString) return undefined;
  const d = new Date(isoString);
  return new Date(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate(), d.getUTCHours(), d.getUTCMinutes());
}

// Helper to pull HH:MM from an ISO UTC string
function hydrateUTCTime(isoString: string | null | undefined): string {
  if (!isoString) return "00:00";
  const d = new Date(isoString);
  const hh = d.getUTCHours().toString().padStart(2, "0");
  const mm = d.getUTCMinutes().toString().padStart(2, "0");
  return `${hh}:${mm}`;
}

function formatMoney(moneyObj: any) {
  if (!moneyObj) return "";
  const amt = typeof moneyObj.amount === 'number' ? moneyObj.amount / 100 : 0;
  const cur = moneyObj.currency?.code || moneyObj.currency || "USD";
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: cur }).format(amt);
}

export function DomainSettingsControls({ domain }: { domain: DomainDetail }) {
  const setDropCatch = useSetDropCatch();
  const unsetDropCatch = useUnsetDropCatch();
  const updateDomain = useUpdateDomain();

  const [popoverOpen, setPopoverOpen] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const isConfigured = (domain.GrandFathering?.Amount || 0) > 0;
  const [gfAmount, setGfAmount] = useState(domain.GrandFathering?.Amount?.toString() || "0");
  const [gfCurrency, setGfCurrency] = useState(domain.GrandFathering?.Currency || "USD");
  const [gfCondition, setGfCondition] = useState(domain.GrandFathering?.ExpiryCondition || "transfer");
  const [gfDate, setGfDate] = useState<Date | undefined>(hydrateUTCDate(domain.GrandFathering?.VoidDate));
  const [gfTime, setGfTime] = useState<string>(hydrateUTCTime(domain.GrandFathering?.VoidDate));

  // Reset state when popover opens
  const handleOpenChange = (open: boolean) => {
    setPopoverOpen(open);
    if (open) {
      setIsEditing(!isConfigured);
      setGfAmount(domain.GrandFathering?.Amount?.toString() || "0");
      setGfCurrency(domain.GrandFathering?.Currency || "USD");
      setGfCondition(domain.GrandFathering?.ExpiryCondition || "transfer");
      setGfDate(hydrateUTCDate(domain.GrandFathering?.VoidDate));
      setGfTime(hydrateUTCTime(domain.GrandFathering?.VoidDate));
    }
  };

  const handleDropCatchChange = async (checked: boolean) => {
    try {
      if (checked) {
        await setDropCatch.mutateAsync(domain.Name);
        toast.success("DropCatch enabled");
      } else {
        await unsetDropCatch.mutateAsync(domain.Name);
        toast.success("DropCatch disabled");
      }
    } catch (error: any) {
      toast.error(error?.response?.data?.error || "Failed to update DropCatch");
    }
  };

  const handleSaveGF = async () => {
    try {
      const payload = { ...domain };

      let voidDateISO = null;
      if (gfCondition === "date" && gfDate) {
        // Construct UTC exactly from the chosen visual YYYY/MM/DD and HH:MM
        const [hours, minutes] = gfTime.split(':').map(Number);
        const utcDate = new Date(Date.UTC(gfDate.getFullYear(), gfDate.getMonth(), gfDate.getDate(), hours || 0, minutes || 0));
        voidDateISO = utcDate.toISOString();
      }
      
      const newGF = {
        Amount: parseInt(gfAmount, 10) || 0,
        Currency: gfCurrency,
        ExpiryCondition: gfCondition,
        VoidDate: voidDateISO,
      };

      payload.GrandFathering = newGF;

      await updateDomain.mutateAsync({ name: domain.Name, payload });
      toast.success("Fixed price override settings updated");
      setPopoverOpen(false);
    } catch (error: any) {
      toast.error(error?.response?.data?.error || "Failed to update fixed price override");
    }
  };

  const handleRemoveGF = async () => {
    try {
      const payload = { ...domain };
      payload.GrandFathering = { Amount: 0, Currency: "", ExpiryCondition: "", VoidDate: null };
      await updateDomain.mutateAsync({ name: domain.Name, payload });
      toast.success("Fixed price override removed");
      setPopoverOpen(false);
    } catch (error: any) {
      toast.error(error?.response?.data?.error || "Failed to remove fixed price override");
    }
  };

  return (
    <div className="flex items-center gap-4 bg-muted/50 px-4 py-2 rounded-lg border border-border/50">
      <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Overrides</span>
      <div className="h-4 w-px bg-border/50"></div>
      
      <div className="flex items-center space-x-2">
        <Switch 
          id="dropcatch-switch" 
          checked={domain.DropCatch} 
          onCheckedChange={handleDropCatchChange}
          disabled={setDropCatch.isPending || unsetDropCatch.isPending}
        />
        <Label htmlFor="dropcatch-switch" className="cursor-pointer font-medium">DropCatch</Label>
      </div>

      <div className="h-4 w-px bg-border"></div>

      <Popover open={popoverOpen} onOpenChange={handleOpenChange}>
        <PopoverTrigger asChild>
          <Button variant="outline" size="sm" className="h-8 gap-2">
            <Settings2 className="w-4 h-4" />
            Fixed Price
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-80" align="end">
          <div className="space-y-4">
            <h4 className="font-medium leading-none">Fixed Renewal Price</h4>
            {!isEditing && isConfigured ? (
              <>
                <div className="text-sm space-y-2 border rounded-md p-3 bg-muted/30">
                  <div className="flex justify-between items-center">
                    <span className="text-muted-foreground">Price</span>
                    <span className="font-medium">{domain.GrandFathering?.Amount} {domain.GrandFathering?.Currency}</span>
                  </div>
                  <div className="flex justify-between items-center gap-4 text-right">
                    <span className="text-muted-foreground whitespace-nowrap">Valid until</span>
                    <span className="font-medium">
                      {domain.GrandFathering?.ExpiryCondition === 'transfer' ? 'domain is transferred' :
                       domain.GrandFathering?.ExpiryCondition === 'delete' ? 'domain is deleted' :
                       domain.GrandFathering?.ExpiryCondition === 'date' ? 'specific date' :
                       domain.GrandFathering?.ExpiryCondition}
                    </span>
                  </div>
                  {domain.GrandFathering?.VoidDate && (
                    <div className="flex justify-between items-center text-right gap-4">
                      <span className="text-muted-foreground whitespace-nowrap">Specific date</span>
                      <span className="font-medium text-xs">
                        {new Date(domain.GrandFathering.VoidDate).toISOString().replace('T', ' ').substring(0, 16)} UTC
                      </span>
                    </div>
                  )}
                </div>
                <div className="flex gap-2">
                  <Button variant="outline" className="flex-1" onClick={() => setIsEditing(true)}>Edit</Button>
                  <Button variant="destructive" className="flex-1" onClick={handleRemoveGF} disabled={updateDomain.isPending}>
                    {updateDomain.isPending ? "Removing..." : "Remove"}
                  </Button>
                </div>
              </>
            ) : (
              <>
                <div className="space-y-2">
                  <div className="grid grid-cols-2 gap-2">
                    <div className="space-y-1">
                      <Label htmlFor="gf-amount">Price</Label>
                      <Input 
                        id="gf-amount" 
                        type="number" 
                        min="0"
                        value={gfAmount} 
                        onChange={e => setGfAmount(e.target.value)} 
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="gf-currency">Currency</Label>
                      <Input 
                        id="gf-currency" 
                        value={gfCurrency} 
                        onChange={e => setGfCurrency(e.target.value.toUpperCase())} 
                        maxLength={3}
                      />
                    </div>
                  </div>
                  <div className="space-y-1">
                    <Label>Valid until</Label>
                    <Select value={gfCondition} onValueChange={setGfCondition}>
                      <SelectTrigger>
                        <SelectValue placeholder="Select condition" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="transfer">Domain is transferred</SelectItem>
                        <SelectItem value="delete">Domain is deleted</SelectItem>
                        <SelectItem value="date">Specific date</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  {gfCondition === "date" && (
                    <div className="pt-2 border-t mt-3">
                      <p className="text-[0.8rem] font-medium mb-2 text-muted-foreground">Select Date and Time (UTC)</p>
                      <div className="grid grid-cols-5 gap-2">
                        <div className="col-span-3">
                          <Popover>
                            <PopoverTrigger asChild>
                              <Button
                                variant="outline"
                                className={cn(
                                  "w-full justify-start text-left font-normal",
                                  !gfDate && "text-muted-foreground"
                                )}
                              >
                                <CalendarIcon className="mr-2 h-4 w-4" />
                                {gfDate ? format(gfDate, "PPP") : <span>Pick date</span>}
                              </Button>
                            </PopoverTrigger>
                            <PopoverContent className="w-auto p-0" align="start">
                              <Calendar
                                mode="single"
                                selected={gfDate}
                                onSelect={setGfDate}
                                initialFocus
                              />
                            </PopoverContent>
                          </Popover>
                        </div>
                        <div className="col-span-2">
                          <Input 
                            type="time" 
                            step="60"
                            value={gfTime} 
                            onChange={(e) => setGfTime(e.target.value)} 
                            className="bg-background text-center w-full"
                          />
                        </div>
                      </div>
                    </div>
                  )}
                </div>
                <div className="flex gap-2">
                  {isConfigured && (
                    <Button variant="outline" onClick={() => setIsEditing(false)}>Cancel</Button>
                  )}
                  <Button className="flex-1" onClick={handleSaveGF} disabled={updateDomain.isPending}>
                    {updateDomain.isPending ? "Saving..." : "Save Settings"}
                  </Button>
                </div>
              </>
            )}
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}
