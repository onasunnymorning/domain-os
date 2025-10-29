// Centralized labels and descriptions for Domain EPP statuses and RGP (Grace Period) states

export const STATUS_LABELS: Record<string, string> = {
  OK: "OK",
  Inactive: "Inactive",
  ClientTransferProhibited: "Client Transfer Prohibited",
  ClientUpdateProhibited: "Client Update Prohibited",
  ClientDeleteProhibited: "Client Delete Prohibited",
  ClientRenewProhibited: "Client Renew Prohibited",
  ClientHold: "Client Hold",
  ServerTransferProhibited: "Server Transfer Prohibited",
  ServerUpdateProhibited: "Server Update Prohibited",
  ServerDeleteProhibited: "Server Delete Prohibited",
  ServerRenewProhibited: "Server Renew Prohibited",
  ServerHold: "Server Hold",
  PendingCreate: "Pending Create",
  PendingRenew: "Pending Renew",
  PendingTransfer: "Pending Transfer",
  PendingUpdate: "Pending Update",
  PendingRestore: "Pending Restore",
  PendingDelete: "Pending Delete",
};

export const STATUS_DESCRIPTIONS: Record<string, string> = {
  OK: "No prohibitions currently set.",
  Inactive: "No nameservers attached; domain may not resolve.",
  ClientTransferProhibited: "Registrar blocks transfer operations.",
  ClientUpdateProhibited: "Registrar blocks update operations.",
  ClientDeleteProhibited: "Registrar blocks delete operations.",
  ClientRenewProhibited: "Registrar blocks renew operations.",
  ClientHold: "Registrar requests DNS suspension (no resolution).",
  ServerTransferProhibited: "Registry blocks transfer operations.",
  ServerUpdateProhibited: "Registry blocks update operations.",
  ServerDeleteProhibited: "Registry blocks delete operations.",
  ServerRenewProhibited: "Registry blocks renew operations.",
  ServerHold: "Registry suspends DNS publishing (no resolution).",
  PendingCreate: "Create request accepted; awaiting completion.",
  PendingRenew: "Renewal in progress; finalization pending.",
  PendingTransfer: "Transfer requested; awaiting approval/timeout.",
  PendingUpdate: "Update submitted; awaiting completion.",
  PendingRestore: "Restore requested; awaiting report/processing.",
  PendingDelete: "Deletion pending; may progress to purge.",
};

export const RGP_LABELS: Record<string, string> = {
  addPeriodEnd: "Add GP",
  autoRenewPeriodEnd: "Auto-Renew GP",
  renewPeriodEnd: "Renew GP",
  transferLockPeriodEnd: "Transfer Lock",
  redemptionPeriodEnd: "Redemption GP",
  purgeDate: "Purge Scheduled",
};

export const RGP_DESCRIPTIONS: Record<string, string> = {
  "Add GP": "Grace period after create; deletes typically refunded.",
  "Auto-Renew GP": "Grace period following auto-renew; can revert/adjust.",
  "Renew GP": "Grace period after explicit renew; can adjust.",
  "Transfer Lock": "Lock applied after transfer to prevent immediate changes.",
  "Redemption GP": "Deleted but restorable for a limited time.",
  "Purge Scheduled": "Domain scheduled to be removed from the registry.",
};
