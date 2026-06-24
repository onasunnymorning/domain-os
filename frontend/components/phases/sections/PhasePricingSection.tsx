'use client';

import { Phase, Price } from '@/lib/types/phase';
import { useState } from 'react';
import { useAddPrice, useDeletePrice } from '@/lib/hooks/usePhases';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { DollarSign, Plus, Trash2 } from 'lucide-react';

interface PhasePricingSectionProps {
  phase: Phase;
  tldName: string;
  onRefetch: () => void;
}

/** Get currency symbol for display */
function getCurrencySymbol(currency: string): string {
  const symbols: Record<string, string> = {
    USD: '$', EUR: '€', GBP: '£', JPY: '¥', CHF: 'Fr', CAD: 'C$', AUD: 'A$',
    NZD: 'NZ$', CNY: '¥', INR: '₹', KRW: '₩', SGD: 'S$', HKD: 'HK$',
    BRL: 'R$', MXN: '$', ZAR: 'R', SEK: 'kr', NOK: 'kr', DKK: 'kr',
    PLN: 'zł', CZK: 'Kč', TRY: '₺', RUB: '₽',
  };
  return symbols[currency?.toUpperCase()] || currency;
}

/** Format cents to dollar string */
function formatAmount(cents: number, currency: string): string {
  return `${getCurrencySymbol(currency)}${(cents / 100).toFixed(2)}`;
}

export function PhasePricingSection({ phase, tldName, onRefetch }: PhasePricingSectionProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [showAddForm, setShowAddForm] = useState(false);
  const [newPrice, setNewPrice] = useState({
    currency: '',
    registrationAmount: '',
    renewalAmount: '',
    transferAmount: '',
    restoreAmount: '',
  });

  const { mutate: addPrice, isPending: isAdding } = useAddPrice(tldName, phase.name);
  const { mutate: deletePrice, isPending: isDeleting } = useDeletePrice(tldName, phase.name);

  const prices = phase.prices || [];

  const handleAddPrice = () => {
    if (!newPrice.currency || !newPrice.registrationAmount) return;
    addPrice({
      currency: newPrice.currency.toUpperCase(),
      registrationAmount: parseInt(newPrice.registrationAmount),
      renewalAmount: parseInt(newPrice.renewalAmount),
      transferAmount: parseInt(newPrice.transferAmount),
      restoreAmount: parseInt(newPrice.restoreAmount),
    }, {
      onSuccess: () => {
        setNewPrice({ currency: '', registrationAmount: '', renewalAmount: '', transferAmount: '', restoreAmount: '' });
        setShowAddForm(false);
        onRefetch();
      },
    });
  };

  const handleDeletePrice = (currency: string) => {
    deletePrice(currency, { onSuccess: onRefetch });
  };

  return (
    <div className="space-y-3">
      {/* Section Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <DollarSign className="h-4 w-4 text-orange-600" />
          <span className="text-sm font-semibold">Pricing</span>
          {prices.length > 0 && (
            <Badge variant="secondary" className="text-xs">
              {prices.length} {prices.length === 1 ? 'currency' : 'currencies'}
            </Badge>
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

      {/* Pricing Table */}
      {prices.length > 0 ? (
        <div className="rounded-lg border overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-muted/40">
                <th className="text-left px-3 py-2 text-xs font-medium text-muted-foreground">Currency</th>
                <th className="text-right px-3 py-2 text-xs font-medium text-muted-foreground">Registration</th>
                <th className="text-right px-3 py-2 text-xs font-medium text-muted-foreground">Renewal</th>
                <th className="text-right px-3 py-2 text-xs font-medium text-muted-foreground">Transfer</th>
                <th className="text-right px-3 py-2 text-xs font-medium text-muted-foreground">Restore</th>
                {isEditing && <th className="w-8" />}
              </tr>
            </thead>
            <tbody className="divide-y divide-border/50">
              {prices.map((price) => (
                <tr key={price.currency} className="hover:bg-muted/20 transition-colors">
                  <td className="px-3 py-2.5">
                    <span className="font-semibold text-xs uppercase tracking-wide text-orange-700 dark:text-orange-400">
                      {price.currency}
                    </span>
                  </td>
                  <td className="text-right px-3 py-2.5 font-mono font-semibold">
                    {formatAmount(price.registrationAmount, price.currency)}
                  </td>
                  <td className="text-right px-3 py-2.5 font-mono font-semibold">
                    {formatAmount(price.renewalAmount, price.currency)}
                  </td>
                  <td className="text-right px-3 py-2.5 font-mono font-semibold">
                    {formatAmount(price.transferAmount, price.currency)}
                  </td>
                  <td className="text-right px-3 py-2.5 font-mono font-semibold">
                    {formatAmount(price.restoreAmount, price.currency)}
                  </td>
                  {isEditing && (
                    <td className="px-2">
                      <Button
                        size="sm"
                        variant="ghost"
                        className="h-6 w-6 p-0 text-destructive hover:text-destructive"
                        onClick={() => handleDeletePrice(price.currency)}
                        disabled={isDeleting}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="text-sm text-muted-foreground italic py-2">
          No pricing configured. Click Edit → Add Currency to set prices.
        </div>
      )}

      {/* Add Currency Form */}
      {isEditing && !showAddForm && (
        <Button
          size="sm"
          variant="outline"
          className="h-7 text-xs"
          onClick={() => setShowAddForm(true)}
        >
          <Plus className="h-3.5 w-3.5 mr-1" />
          Add Currency
        </Button>
      )}

      {showAddForm && (
        <div className="rounded-lg border p-3 space-y-3 bg-muted/10">
          <div className="text-xs font-medium">Add New Currency</div>
          <div className="grid grid-cols-5 gap-2">
            <div>
              <label className="text-[10px] text-muted-foreground uppercase">Currency</label>
              <input
                type="text"
                maxLength={3}
                value={newPrice.currency}
                onChange={(e) => setNewPrice({ ...newPrice, currency: e.target.value.toUpperCase() })}
                placeholder="USD"
                className="mt-0.5 w-full px-2 py-1.5 border rounded text-sm uppercase bg-background"
              />
            </div>
            {(['registrationAmount', 'renewalAmount', 'transferAmount', 'restoreAmount'] as const).map((field) => {
              const label = field.replace('Amount', '');
              const value = newPrice[field];
              return (
                <div key={field}>
                  <label className="text-[10px] text-muted-foreground uppercase">{label}</label>
                  <div className="relative mt-0.5">
                    <input
                      type="number"
                      min="0"
                      value={value}
                      onChange={(e) => setNewPrice({ ...newPrice, [field]: e.target.value })}
                      placeholder="0"
                      className="w-full px-2 py-1.5 border rounded text-sm bg-background"
                    />
                    {/* Live dollar preview */}
                    {value && newPrice.currency && (
                      <div className="absolute -bottom-4 left-0 text-[10px] text-muted-foreground font-mono">
                        {formatAmount(parseInt(value) || 0, newPrice.currency)}
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
          <div className="flex gap-2 pt-2">
            <Button
              size="sm"
              className="h-7 text-xs"
              onClick={handleAddPrice}
              disabled={isAdding || !newPrice.currency || !newPrice.registrationAmount}
            >
              {isAdding ? 'Adding...' : 'Add'}
            </Button>
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs"
              onClick={() => { setShowAddForm(false); setNewPrice({ currency: '', registrationAmount: '', renewalAmount: '', transferAmount: '', restoreAmount: '' }); }}
            >
              Cancel
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
