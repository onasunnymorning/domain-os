'use client';

import { Phase } from '@/lib/types/phase';
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
        changes.push({ field, old: oldValue, new: newValue, type: 'added' });
      } else if (oldValue !== undefined && newValue === undefined) {
        changes.push({ field, old: oldValue, new: newValue, type: 'removed' });
      } else {
        changes.push({ field, old: oldValue, new: newValue, type: 'changed' });
      }
    }
  });

  // Compare pricing
  const oldPrices = compareWith.prices || [];
  const newPrices = phase.prices || [];
  
  const priceChanges = new Map<string, { old?: number; new?: number }>();
  oldPrices.forEach(p => priceChanges.set(p.currency, { old: p.amount }));
  newPrices.forEach(p => {
    const existing = priceChanges.get(p.currency);
    if (existing) {
      existing.new = p.amount;
    } else {
      priceChanges.set(p.currency, { new: p.amount });
    }
  });

  if (changes.length === 0 && priceChanges.size === 0) {
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

  const formatValue = (value: any) => {
    if (value === undefined || value === null) return '-';
    if (typeof value === 'boolean') return value ? 'Yes' : 'No';
    if (typeof value === 'number') return value.toString();
    return value;
  };

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
            {changes.length + priceChanges.size} change{changes.length + priceChanges.size !== 1 ? 's' : ''}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {/* Policy Changes */}
        {changes.length > 0 && (
          <div className="space-y-2">
            <div className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              Policy Changes
            </div>
            {changes.map((change, idx) => (
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
                      {formatValue(change.old)}
                    </span>
                  )}
                  {change.type !== 'removed' && (
                    <>
                      {change.type === 'changed' && <ArrowRight className="h-3 w-3" />}
                      <span className="font-medium">{formatValue(change.new)}</span>
                    </>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Price Changes */}
        {priceChanges.size > 0 && (
          <div className="space-y-2">
            <div className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              Pricing Changes
            </div>
            {Array.from(priceChanges.entries()).map(([currency, prices], idx) => {
              const hasOld = prices.old !== undefined;
              const hasNew = prices.new !== undefined;
              const changeType = !hasOld ? 'added' : !hasNew ? 'removed' : 'changed';

              return (
                <div
                  key={idx}
                  className="flex items-center justify-between text-sm p-2 rounded-lg border"
                >
                  <div className="flex items-center gap-2">
                    {changeType === 'added' && <Plus className="h-3 w-3 text-green-600" />}
                    {changeType === 'removed' && <Minus className="h-3 w-3 text-red-600" />}
                    {changeType === 'changed' && <ArrowRight className="h-3 w-3 text-orange-600" />}
                    <span className="font-medium">{currency}</span>
                  </div>
                  <div className="flex items-center gap-2 text-xs">
                    {hasOld && (
                      <span className="text-muted-foreground line-through">
                        {(prices.old! / 100).toFixed(2)}
                      </span>
                    )}
                    {hasOld && hasNew && <ArrowRight className="h-3 w-3" />}
                    {hasNew && (
                      <span className="font-medium">{(prices.new! / 100).toFixed(2)}</span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
