import { NextResponse } from 'next/server';

// Liveness/readiness target for load balancers (declared in deploy/contract.json
// as the frontend's health_check). Deliberately unauthenticated and outside the
// Auth0 gate, and it renders no React so an ALB probe stays cheap.
export const dynamic = 'force-dynamic';

export function GET() {
  return NextResponse.json({ status: 'ok' });
}
