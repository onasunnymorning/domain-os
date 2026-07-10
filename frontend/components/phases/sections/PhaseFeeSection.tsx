'use client';

import { Phase, Fee } from '@/lib/types/phase';
import { useState } from 'react';
import { useAddFee, useDeleteFee } from '@/lib/hooks/usePhases';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Tag, Plus, Trash2 } from 'lucide-react';

interface PhaseFeeSectionProps {
  phase: Phase;
  tldName: string;
  onRefetch: () => void;
}

function getCurrencySymbol(currency: string): string {
  const symbols: Record<string, string> = {
    USD: '$', EUR: '€', GBP: '£', JPY: '¥', CHF: 'Fr',
  };
  return symbols[currency?.toUpperCase()] || currency;
}

function formatAmount(cents: number, currency: string): string {
  return `${getCurrencySymbol(currency)}${(cents / 100).toFixed(2)}`;
}

export function PhaseFeeSection({ phase, tldName, onRefetch }: PhaseFeeSectionProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [showAddForm, setShowAddForm] = useState(false);
  const [newFee, setNewFee] = useState({
    name: '',
    currency: '',
    amount: '',
    refundable: false,
  });

  const { mutate: addFee, isPending: isAdding } = useAddFee(tldName, phase.name);
  const { mutate: deleteFee, isPending: isDeleting } = useDeleteFee(tldName, phase.name);

  const fees = phase.fees || [];

  const handleAddFee = () => {
    if (!newFee.name || !newFee.currency || !newFee.amount) return;
    addFee({
      name: newFee.name,
      currency: newFee.currency.toUpperCase(),
      amount: parseInt(newFee.amount),
      refundable: newFee.refundable,
    }, {
      onSuccess: () => {
        setNewFee({ name: '', currency: '', amount: '', refundable: false });
        setShowAddForm(false);
        onRefetch();
      },
    });
  };

  const handleDeleteFee = (feeName: string, currency: string) => {
    deleteFee({ feeName, currency }, { onSuccess: onRefetch });
  };

  return (
    <div className="space-y-3">
      {/* Section Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Tag className="h-4 w-4 text-orange-600" />
          <span className="text-sm font-semibold">Fees</span>
          {fees.length > 0 && (
            <Badge variant="secondary" className="text-xs">{fees.length}</Badge>
          )}
        </div>
        <Button
          size="sm"
          variant="ghost"
          className="h-7 text-xs"
          onClick={() => setIsEditing(!isEditing)}
        >
          {isEditing ? 'Done' : 'Edit'}
        </Button>
      </div>

      {/* Fee List */}
      {fees.length > 0 ? (
        <div className="space-y-2">
          {fees.map((fee, index) => (
            <div key={index} className="flex items-center justify-between p-3 rounded-lg bg-muted/30 border border-border/40">
              <div className="flex flex-col flex-1">
                <span className="text-sm font-medium">{fee.name}</span>
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <span>{fee.currency}</span>
                  {fee.refundable && (
                    <Badge variant="outline" className="text-[10px] h-4 border-green-500/50 text-green-600">
                      Refundable
                    </Badge>
                  )}
                </div>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-lg font-semibold font-mono">
                  {formatAmount(fee.amount, fee.currency)}
                </span>
                {isEditing && (
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-6 w-6 p-0 text-destructive hover:text-destructive"
                    onClick={() => handleDeleteFee(fee.name, fee.currency)}
                    disabled={isDeleting}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                )}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="text-sm text-muted-foreground italic py-2">
          No fees configured.
        </div>
      )}

      {/* Add Fee */}
      {isEditing && !showAddForm && (
        <Button
          size="sm"
          variant="outline"
          className="h-7 text-xs"
          onClick={() => setShowAddForm(true)}
        >
          <Plus className="h-3.5 w-3.5 mr-1" />
          Add Fee
        </Button>
      )}

      {showAddForm && (
        <div className="rounded-lg border p-3 space-y-3 bg-muted/10">
          <div className="text-xs font-medium">Add New Fee</div>
          <div className="grid grid-cols-2 gap-2">
            <div className="col-span-2">
              <label className="text-[10px] text-muted-foreground uppercase">Fee Name</label>
              <input
                type="text"
                value={newFee.name}
                onChange={(e) => setNewFee({ ...newFee, name: e.target.value })}
                placeholder="e.g., application_fee"
                className="mt-0.5 w-full px-2 py-1.5 border rounded text-sm bg-background"
              />
            </div>
            <div>
              <label className="text-[10px] text-muted-foreground uppercase">Currency</label>
              <input
                type="text"
                maxLength={3}
                value={newFee.currency}
                onChange={(e) => setNewFee({ ...newFee, currency: e.target.value.toUpperCase() })}
                placeholder="USD"
                className="mt-0.5 w-full px-2 py-1.5 border rounded text-sm uppercase bg-background"
              />
            </div>
            <div>
              <label className="text-[10px] text-muted-foreground uppercase">Amount (cents)</label>
              <div className="relative mt-0.5">
                <input
                  type="number"
                  min="0"
                  value={newFee.amount}
                  onChange={(e) => setNewFee({ ...newFee, amount: e.target.value })}
                  placeholder="0"
                  className="w-full px-2 py-1.5 border rounded text-sm bg-background"
                />
                {newFee.amount && newFee.currency && (
                  <div className="absolute -bottom-4 left-0 text-[10px] text-muted-foreground font-mono">
                    {formatAmount(parseInt(newFee.amount) || 0, newFee.currency)}
                  </div>
                )}
              </div>
            </div>
            <div className="col-span-2 pt-1">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={newFee.refundable}
                  onChange={(e) => setNewFee({ ...newFee, refundable: e.target.checked })}
                  className="h-4 w-4 rounded border-gray-300"
                />
                <span className="text-sm">Refundable</span>
              </label>
            </div>
          </div>
          <div className="flex gap-2 pt-1">
            <Button
              size="sm"
              className="h-7 text-xs"
              onClick={handleAddFee}
              disabled={isAdding || !newFee.name || !newFee.currency || !newFee.amount}
            >
              {isAdding ? 'Adding...' : 'Add Fee'}
            </Button>
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs"
              onClick={() => { setShowAddForm(false); setNewFee({ name: '', currency: '', amount: '', refundable: false }); }}
            >
              Cancel
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
