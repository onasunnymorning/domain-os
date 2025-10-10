'use client';

import { Phase, Price, Fee } from '@/lib/types/phase';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { ArrowRight, Plus, Minus, AlertCircle } from 'lucide-react';

interface PhaseConfigDiffProps {
  phase: Phase;
  compareWith: Phase | null;
}

export function PhaseConfigDiff({ phase, compareWith }: PhaseConfigDiffProps) {
  if (!compareWith) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Configuration Changes</CardTitle>
          <CardDescription>No previous phase to compare with</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-sm text-muted-foreground italic">
            This is the first phase or no previous phase selected
          </div>
        </CardContent>
      </Card>
    );
  }

  const changes: Array<{
    field: string;
    old: any;
    new: any;
    type: 'added' | 'removed' | 'changed';
    category: 'policy' | 'pricing' | 'fees';
  }> = [];

  // Compare policy fields
  const policyFields: Array<keyof typeof phase.policy> = [
    'minLabelLength',
    'maxLabelLength',
    'registrationGP',
    'renewalGP',
    'autoRenewalGP',
    'transferGP',
    'redemptionGP',
    'pendingdeleteGP',
    'transferLockPeriod',
    'maxHorizon',
    'allowAutorenew',
    'requiresValidation',
    'baseCurrency',
  ];

  policyFields.forEach(field => {
    const oldValue = compareWith.policy[field];
    const newValue = phase.policy[field];

    if (oldValue !== newValue) {
      if (oldValue === undefined && newValue !== undefined) {
        changes.push({ field, old: oldValue, new: newValue, type: 'added', category: 'policy' });
      } else if (oldValue !== undefined && newValue === undefined) {
        changes.push({ field, old: oldValue, new: newValue, type: 'removed', category: 'policy' });
      } else {
        changes.push({ field, old: oldValue, new: newValue, type: 'changed', category: 'policy' });
      }
    }
  });

  // Compare pricing (now with registrationAmount, renewalAmount, etc.)
  const oldPrices = compareWith.prices || [];
  const newPrices = phase.prices || [];
  
  const priceMap = new Map<string, { old?: Price; new?: Price }>();
  oldPrices.forEach(p => priceMap.set(p.currency, { old: p }));
  newPrices.forEach(p => {
    const existing = priceMap.get(p.currency);
    if (existing) {
      existing.new = p;
    } else {
      priceMap.set(p.currency, { new: p });
    }
  });

  // Detect price changes for each currency
  priceMap.forEach((prices, currency) => {
    const old = prices.old;
    const neu = prices.new;

    if (!old && neu) {
      // Currency added
      changes.push({
        field: `${currency} (all prices)`,
        old: undefined,
        new: neu,
        type: 'added',
        category: 'pricing'
      });
    } else if (old && !neu) {
      // Currency removed
      changes.push({
        field: `${currency} (all prices)`,
        old: old,
        new: undefined,
        type: 'removed',
        category: 'pricing'
      });
    } else if (old && neu) {
      // Check individual price types
      const priceTypes: Array<keyof Price> = ['registrationAmount', 'renewalAmount', 'transferAmount', 'restoreAmount'];
      priceTypes.forEach(type => {
        if (old[type] !== neu[type]) {
          changes.push({
            field: `${currency} ${type.replace('Amount', '')}`,
            old: old[type],
            new: neu[type],
            type: 'changed',
            category: 'pricing'
          });
        }
      });
    }
  });

  // Compare fees
  const oldFees = compareWith.fees || [];
  const newFees = phase.fees || [];
  
  const feeMap = new Map<string, { old?: Fee; new?: Fee }>();
  oldFees.forEach(f => feeMap.set(`${f.name}-${f.currency}`, { old: f }));
  newFees.forEach(f => {
    const key = `${f.name}-${f.currency}`;
    const existing = feeMap.get(key);
    if (existing) {
      existing.new = f;
    } else {
      feeMap.set(key, { new: f });
    }
  });

  // Detect fee changes
  feeMap.forEach((fees, key) => {
    const old = fees.old;
    const neu = fees.new;

    if (!old && neu) {
      // Fee added
      changes.push({
        field: `${neu.name} (${neu.currency})`,
        old: undefined,
        new: neu.amount,
        type: 'added',
        category: 'fees'
      });
    } else if (old && !neu) {
      // Fee removed
      changes.push({
        field: `${old.name} (${old.currency})`,
        old: old.amount,
        new: undefined,
        type: 'removed',
        category: 'fees'
      });
    } else if (old && neu) {
      // Check if amount or refundable changed
      if (old.amount !== neu.amount) {
        changes.push({
          field: `${neu.name} (${neu.currency}) amount`,
          old: old.amount,
          new: neu.amount,
          type: 'changed',
          category: 'fees'
        });
      }
      if (old.refundable !== neu.refundable) {
        changes.push({
          field: `${neu.name} (${neu.currency}) refundable`,
          old: old.refundable,
          new: neu.refundable,
          type: 'changed',
          category: 'fees'
        });
      }
    }
  });

  if (changes.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Configuration Changes</CardTitle>
          <CardDescription>Comparing with {compareWith.name}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-sm text-muted-foreground italic">
            No configuration changes detected
          </div>
        </CardContent>
      </Card>
    );
  }

  const formatFieldName = (field: string) => {
    return field
      .replace(/([A-Z])/g, ' $1')
      .replace(/GP$/, ' (Grace Period)')
      .trim()
      .split(' ')
      .map(word => word.charAt(0).toUpperCase() + word.slice(1))
      .join(' ');
  };

  const formatValue = (value: any, category: string) => {
    if (value === undefined || value === null) return '-';
    if (typeof value === 'boolean') return value ? 'Yes' : 'No';
    if (category === 'pricing' || category === 'fees') {
      // Handle price amounts (convert from cents to dollars)
      if (typeof value === 'number') return `$${(value / 100).toFixed(2)}`;
      // Handle Price object
      if (typeof value === 'object' && 'registrationAmount' in value) {
        return `Reg: $${(value.registrationAmount / 100).toFixed(2)}, Renew: $${(value.renewalAmount / 100).toFixed(2)}, Transfer: $${(value.transferAmount / 100).toFixed(2)}, Restore: $${(value.restoreAmount / 100).toFixed(2)}`;
      }
    }
    if (typeof value === 'number') return value.toString();
    return value;
  };

  const policyChanges = changes.filter(c => c.category === 'policy');
  const pricingChanges = changes.filter(c => c.category === 'pricing');
  const feeChanges = changes.filter(c => c.category === 'fees');

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-sm">Configuration Changes</CardTitle>
            <CardDescription>
              Comparing {compareWith.name} → {phase.name}
            </CardDescription>
          </div>
          <Badge variant="outline">
            {changes.length} change{changes.length !== 1 ? 's' : ''}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Policy Changes */}
        {policyChanges.length > 0 && (
          <div className="space-y-2">
            <div className="text-xs font-semibold text-orange-700 uppercase tracking-wide">
              Policy Changes ({policyChanges.length})
            </div>
            {policyChanges.map((change, idx) => (
              <div
                key={idx}
                className="flex items-center justify-between text-sm p-2 rounded-lg border"
              >
                <div className="flex items-center gap-2">
                  {change.type === 'added' && <Plus className="h-3 w-3 text-green-600" />}
                  {change.type === 'removed' && <Minus className="h-3 w-3 text-red-600" />}
                  {change.type === 'changed' && <ArrowRight className="h-3 w-3 text-orange-600" />}
                  <span className="font-medium">{formatFieldName(change.field)}</span>
                </div>
                <div className="flex items-center gap-2 text-xs">
                  {change.type !== 'added' && (
                    <span className="text-muted-foreground line-through">
                      {formatValue(change.old, 'policy')}
                    </span>
                  )}
                  {change.type !== 'removed' && (
                    <>
                      {change.type === 'changed' && <ArrowRight className="h-3 w-3" />}
                      <span className="font-medium">{formatValue(change.new, 'policy')}</span>
                    </>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Pricing Changes */}
        {pricingChanges.length > 0 && (
          <div className="space-y-2">
            <div className="text-xs font-semibold text-orange-700 uppercase tracking-wide">
              Pricing Changes ({pricingChanges.length})
            </div>
            {pricingChanges.map((change, idx) => (
              <div
                key={idx}
                className="flex items-center justify-between text-sm p-2 rounded-lg border"
              >
                <div className="flex items-center gap-2">
                  {change.type === 'added' && <Plus className="h-3 w-3 text-green-600" />}
                  {change.type === 'removed' && <Minus className="h-3 w-3 text-red-600" />}
                  {change.type === 'changed' && <ArrowRight className="h-3 w-3 text-orange-600" />}
                  <span className="font-medium">{change.field}</span>
                </div>
                <div className="flex items-center gap-2 text-xs">
                  {change.type !== 'added' && (
                    <span className="text-muted-foreground line-through">
                      {formatValue(change.old, 'pricing')}
                    </span>
                  )}
                  {change.type !== 'removed' && (
                    <>
                      {change.type === 'changed' && <ArrowRight className="h-3 w-3" />}
                      <span className="font-medium">{formatValue(change.new, 'pricing')}</span>
                    </>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Fee Changes */}
        {feeChanges.length > 0 && (
          <div className="space-y-2">
            <div className="text-xs font-semibold text-orange-700 uppercase tracking-wide">
              Fee Changes ({feeChanges.length})
            </div>
            {feeChanges.map((change, idx) => (
              <div
                key={idx}
                className="flex items-center justify-between text-sm p-2 rounded-lg border"
              >
                <div className="flex items-center gap-2">
                  {change.type === 'added' && <Plus className="h-3 w-3 text-green-600" />}
                  {change.type === 'removed' && <Minus className="h-3 w-3 text-red-600" />}
                  {change.type === 'changed' && <ArrowRight className="h-3 w-3 text-orange-600" />}
                  <span className="font-medium">{change.field}</span>
                </div>
                <div className="flex items-center gap-2 text-xs">
                  {change.type !== 'added' && (
                    <span className="text-muted-foreground line-through">
                      {formatValue(change.old, 'fees')}
                    </span>
                  )}
                  {change.type !== 'removed' && (
                    <>
                      {change.type === 'changed' && <ArrowRight className="h-3 w-3" />}
                      <span className="font-medium">{formatValue(change.new, 'fees')}</span>
                    </>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
