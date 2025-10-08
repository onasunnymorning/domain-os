# Frontend Architecture & Implementation Roadmap

## Executive Summary

Based on your requirements and limited frontend experience, I recommend **Next.js 14+ with TypeScript** as your frontend framework. This choice balances ease of learning, excellent developer experience, and production readiness.

## Current Backend Analysis

### API Structure
- **Framework**: Go (Gin framework)
- **API Type**: REST with Swagger/OpenAPI documentation
- **Authentication**: Token-based (Bearer tokens)
- **Base URL**: Configurable via environment
- **Port**: 8080 (default)
- **Swagger UI**: Available at `/swagger/index.html`

### Key API Endpoints (Registry Operator Focus)
```
POST   /registry-operators          # Create Registry Operator
GET    /registry-operators          # List Registry Operators (with pagination)
GET    /registry-operators/:ryid    # Get specific Registry Operator
PUT    /registry-operators/:ryid    # Update Registry Operator
DELETE /registry-operators/:ryid    # Delete Registry Operator
```

### Data Model (Registry Operator)
```typescript
interface CreateRegistryOperatorCommand {
  RyID: string;   // Required
  Name: string;   // Required
  Email: string;  // Required
  URL?: string;   // Optional
  Voice?: string; // Optional (E.164 format)
  Fax?: string;   // Optional (E.164 format)
}

interface RegistryOperator {
  RyID: string;
  Name: string;
  Email: string;
  URL?: string;
  Voice?: E164Type;
  Fax?: E164Type;
  CreatedAt: string;
  UpdatedAt: string;
}
```

## Recommended Tech Stack

### 🏆 Primary Recommendation: Next.js 14+ with App Router

**Why Next.js?**
1. **Full-Stack Framework** - Can handle both frontend and backend routes
2. **TypeScript Native** - Excellent type safety and autocomplete
3. **Great DX** - Hot reload, excellent error messages, built-in tooling
4. **Production Ready** - Used by Fortune 500 companies
5. **Easy Learning Curve** - Extensive documentation and tutorials
6. **Server Components** - Built-in server-side rendering for better performance
7. **API Routes** - Can proxy your Go API if needed (CORS handling, etc.)

### Complete Stack

```yaml
Core Framework: Next.js 14+ (React 18+)
Language: TypeScript
Styling: Tailwind CSS + shadcn/ui
Forms: React Hook Form + Zod validation
API Client: TanStack Query (React Query) + Axios
State Management: Zustand (if needed, start without it)
Auth: NextAuth.js (Auth0 adapter available)
Testing: Vitest + React Testing Library
Development: ESLint + Prettier
```

### Alternative Options (Not Recommended for Your Case)

**❌ Plain React (Vite)**
- More configuration needed
- Need to set up routing, SSR, etc.
- More decisions to make

**❌ Vue/Nuxt**
- Smaller ecosystem
- Less enterprise adoption
- Good option, but React has more resources

**❌ Angular**
- Steeper learning curve
- Overkill for this use case
- More opinionated/complex

## Architecture Design

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Browser (User)                        │
└────────────────────────┬────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│              Next.js Frontend (Port 3000)                │
│  ┌─────────────────────────────────────────────────┐    │
│  │  App Router (Pages & Layouts)                   │    │
│  ├─────────────────────────────────────────────────┤    │
│  │  React Components (shadcn/ui)                   │    │
│  ├─────────────────────────────────────────────────┤    │
│  │  TanStack Query (API State Management)          │    │
│  ├─────────────────────────────────────────────────┤    │
│  │  Auth Layer (NextAuth.js - Mock for now)        │    │
│  └─────────────────────────────────────────────────┘    │
└────────────────────────┬────────────────────────────────┘
                         │ HTTP/REST
                         ▼
┌─────────────────────────────────────────────────────────┐
│           Go Backend API (Port 8080)                     │
│  ┌─────────────────────────────────────────────────┐    │
│  │  Gin REST API + Swagger                         │    │
│  ├─────────────────────────────────────────────────┤    │
│  │  Business Logic (Services)                      │    │
│  ├─────────────────────────────────────────────────┤    │
│  │  PostgreSQL Database                            │    │
│  └─────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

### Folder Structure

```
domain-os-frontend/
├── src/
│   ├── app/                          # Next.js App Router
│   │   ├── layout.tsx               # Root layout
│   │   ├── page.tsx                 # Home/Dashboard
│   │   ├── login/
│   │   │   └── page.tsx
│   │   └── registry-operators/
│   │       ├── page.tsx             # List view
│   │       ├── create/
│   │       │   └── page.tsx         # Create form
│   │       └── [ryid]/
│   │           ├── page.tsx         # Detail view
│   │           └── edit/
│   │               └── page.tsx     # Edit form
│   │
│   ├── components/                   # Reusable components
│   │   ├── ui/                      # shadcn/ui components
│   │   ├── forms/
│   │   │   └── RegistryOperatorForm.tsx
│   │   ├── layout/
│   │   │   ├── Header.tsx
│   │   │   ├── Sidebar.tsx
│   │   │   └── DashboardLayout.tsx
│   │   └── registry-operators/
│   │       ├── RegistryOperatorList.tsx
│   │       ├── RegistryOperatorCard.tsx
│   │       └── RegistryOperatorTable.tsx
│   │
│   ├── lib/                         # Utilities
│   │   ├── api/
│   │   │   ├── client.ts           # Axios instance
│   │   │   ├── registry-operators.ts
│   │   │   └── types.ts            # API types
│   │   ├── hooks/
│   │   │   └── useRegistryOperators.ts
│   │   └── utils.ts
│   │
│   ├── types/                       # TypeScript types
│   │   └── index.ts
│   │
│   └── middleware.ts                # Auth middleware
│
├── public/                          # Static files
├── .env.local                       # Environment variables
├── next.config.js
├── tailwind.config.ts
├── tsconfig.json
└── package.json
```

## Implementation Roadmap

### Phase 1: Foundation (Days 1-2) ⏰ 8-10 hours

**Goal**: Get a working Next.js app talking to your Go API

#### Step 1.1: Project Setup
```bash
# Create Next.js project
npx create-next-app@latest domain-os-frontend --typescript --tailwind --app --no-src-dir

# Install core dependencies
cd domain-os-frontend
npm install axios @tanstack/react-query zustand
npm install react-hook-form @hookform/resolvers zod
npm install -D @types/node
```

**Deliverable**: Running Next.js app on `http://localhost:3000`

#### Step 1.2: Environment Configuration
Create `.env.local`:
```env
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_API_TOKEN=your-dev-token-here
```

#### Step 1.3: API Client Setup
Create the API client to communicate with your Go backend.

**Files to create**:
- `lib/api/client.ts` - Axios instance with auth headers
- `lib/api/types.ts` - TypeScript types from your API
- `lib/api/registry-operators.ts` - API methods

**Deliverable**: Working API client that can make requests to Go backend

### Phase 2: UI Foundation (Days 3-4) ⏰ 8-10 hours

**Goal**: Set up design system and basic layout

#### Step 2.1: Install shadcn/ui
```bash
npx shadcn-ui@latest init
npx shadcn-ui@latest add button card form input label table
```

#### Step 2.2: Create Basic Layout
**Files to create**:
- `app/layout.tsx` - Root layout with providers
- `components/layout/DashboardLayout.tsx` - Admin dashboard layout
- `components/layout/Header.tsx` - Top navigation
- `components/layout/Sidebar.tsx` - Side navigation

**Deliverable**: Professional-looking admin layout with navigation

### Phase 3: Registry Operator CRUD (Days 5-7) ⏰ 12-15 hours

**Goal**: Complete CRUD for Registry Operators

#### Step 3.1: List View
- Create `app/registry-operators/page.tsx`
- Implement table with pagination
- Add search/filter functionality
- Show loading and error states

#### Step 3.2: Create Form
- Create `app/registry-operators/create/page.tsx`
- Build form with validation
- Handle API errors
- Success/error notifications

#### Step 3.3: Detail & Edit Views
- Create `app/registry-operators/[ryid]/page.tsx`
- Create `app/registry-operators/[ryid]/edit/page.tsx`
- Implement update functionality
- Add delete functionality with confirmation

**Deliverable**: Full CRUD for Registry Operators

### Phase 4: Mock Authentication (Days 8-9) ⏰ 6-8 hours

**Goal**: Basic auth flow (bypass for now, prepare for Auth0)

#### Step 4.1: Mock Auth Setup
- Create simple login page
- Store mock token in localStorage/cookie
- Implement auth middleware
- Protect routes

#### Step 4.2: Auth UI
- Login page with form
- Logout functionality
- User profile display
- Session persistence

**Deliverable**: Working auth flow (mocked)

### Phase 5: Polish & Testing (Days 10-12) ⏰ 8-10 hours

**Goal**: Production-ready features

#### Step 5.1: Error Handling
- Global error boundary
- API error handling
- User-friendly error messages
- Retry mechanisms

#### Step 5.2: UX Improvements
- Loading skeletons
- Optimistic updates
- Toast notifications
- Form validation feedback

#### Step 5.3: Testing
- Unit tests for utilities
- Component tests
- API mock tests
- E2E critical paths

**Deliverable**: Polished, tested application

### Phase 6: Deployment (Day 13) ⏰ 4-6 hours

**Goal**: Deploy to production

#### Options:
1. **Vercel** (Recommended) - Zero config, Next.js native
2. **Docker** - Containerized deployment
3. **AWS/Azure** - If required by infrastructure

**Deliverable**: Live application accessible via URL

## Total Time Estimate

**50-65 hours total** (~2 weeks full-time or 4-6 weeks part-time)

## Learning Resources

### Essential (Start Here)
1. **Next.js Official Tutorial** (4 hours)
   - https://nextjs.org/learn
   
2. **TypeScript for JavaScript Developers** (3 hours)
   - https://www.typescriptlang.org/docs/handbook/typescript-in-5-minutes.html
   
3. **Tailwind CSS Fundamentals** (2 hours)
   - https://tailwindcss.com/docs

### Recommended (Build Phase)
4. **React Query Tutorial** (3 hours)
   - https://tanstack.com/query/latest/docs/react/overview
   
5. **React Hook Form + Zod** (2 hours)
   - https://react-hook-form.com/get-started
   - https://zod.dev/

6. **shadcn/ui Components** (1 hour)
   - https://ui.shadcn.com/docs

### Video Courses (If Preferred)
- **Next.js 14 Full Course** by Dave Gray (YouTube)
- **React + TypeScript** by Net Ninja (YouTube)

## Quick Start Commands

### Option 1: Manual Setup (Recommended for Learning)

```bash
# 1. Create Next.js app
npx create-next-app@latest domain-os-frontend \
  --typescript \
  --tailwind \
  --app \
  --import-alias "@/*"

cd domain-os-frontend

# 2. Install dependencies
npm install axios @tanstack/react-query
npm install react-hook-form @hookform/resolvers zod
npm install zustand
npm install date-fns # for date formatting

# 3. Install shadcn/ui
npx shadcn-ui@latest init

# 4. Add UI components
npx shadcn-ui@latest add button card form input label table \
  select textarea dialog alert toast

# 5. Create .env.local
cat > .env.local << EOF
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_API_TOKEN=your-dev-token
EOF

# 6. Start development
npm run dev
```

### Option 2: Clone Starter Template (Faster Start)

I can create a complete starter template for you with:
- ✅ All dependencies configured
- ✅ API client ready
- ✅ Layout components
- ✅ Example CRUD for Registry Operators
- ✅ Mock auth setup

Would you like me to create this starter template?

## Code Examples

### 1. API Client (`lib/api/client.ts`)

```typescript
import axios from 'axios';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
const API_TOKEN = process.env.NEXT_PUBLIC_API_TOKEN || '';

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${API_TOKEN}`,
  },
});

// Request interceptor for auth
apiClient.interceptors.request.use(
  (config) => {
    // Get token from localStorage if available (for dynamic auth)
    const token = localStorage.getItem('auth_token') || API_TOKEN;
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor for errors
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Redirect to login
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);
```

### 2. Registry Operator API (`lib/api/registry-operators.ts`)

```typescript
import { apiClient } from './client';
import type { 
  RegistryOperator, 
  CreateRegistryOperatorCommand,
  ListResponse 
} from './types';

export const registryOperatorsApi = {
  list: async (params?: { 
    pagesize?: number; 
    pagecursor?: string;
    ryid_like?: string;
    name_like?: string;
  }): Promise<ListResponse<RegistryOperator>> => {
    const { data } = await apiClient.get('/registry-operators', { params });
    return data;
  },

  getById: async (ryid: string): Promise<RegistryOperator> => {
    const { data } = await apiClient.get(`/registry-operators/${ryid}`);
    return data;
  },

  create: async (command: CreateRegistryOperatorCommand): Promise<RegistryOperator> => {
    const { data } = await apiClient.post('/registry-operators', command);
    return data;
  },

  update: async (ryid: string, operator: RegistryOperator): Promise<RegistryOperator> => {
    const { data } = await apiClient.put(`/registry-operators/${ryid}`, operator);
    return data;
  },

  delete: async (ryid: string): Promise<void> => {
    await apiClient.delete(`/registry-operators/${ryid}`);
  },
};
```

### 3. React Hook (`lib/hooks/useRegistryOperators.ts`)

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { registryOperatorsApi } from '@/lib/api/registry-operators';
import type { CreateRegistryOperatorCommand } from '@/lib/api/types';

export function useRegistryOperators(params?: any) {
  return useQuery({
    queryKey: ['registry-operators', params],
    queryFn: () => registryOperatorsApi.list(params),
  });
}

export function useRegistryOperator(ryid: string) {
  return useQuery({
    queryKey: ['registry-operator', ryid],
    queryFn: () => registryOperatorsApi.getById(ryid),
    enabled: !!ryid,
  });
}

export function useCreateRegistryOperator() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: (data: CreateRegistryOperatorCommand) => 
      registryOperatorsApi.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['registry-operators'] });
    },
  });
}

export function useUpdateRegistryOperator() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: ({ ryid, data }: { ryid: string; data: any }) => 
      registryOperatorsApi.update(ryid, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['registry-operators'] });
    },
  });
}

export function useDeleteRegistryOperator() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: (ryid: string) => registryOperatorsApi.delete(ryid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['registry-operators'] });
    },
  });
}
```

### 4. List Page (`app/registry-operators/page.tsx`)

```typescript
'use client';

import { useState } from 'react';
import { useRegistryOperators } from '@/lib/hooks/useRegistryOperators';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { RegistryOperatorTable } from '@/components/registry-operators/RegistryOperatorTable';
import { PlusIcon } from 'lucide-react';
import Link from 'next/link';

export default function RegistryOperatorsPage() {
  const [search, setSearch] = useState('');
  const { data, isLoading, error } = useRegistryOperators({
    name_like: search,
  });

  if (error) return <div>Error loading registry operators</div>;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold">Registry Operators</h1>
          <p className="text-muted-foreground">
            Manage registry operators in your system
          </p>
        </div>
        <Link href="/registry-operators/create">
          <Button>
            <PlusIcon className="mr-2 h-4 w-4" />
            Create Registry Operator
          </Button>
        </Link>
      </div>

      <div className="flex gap-4">
        <Input
          placeholder="Search by name..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>

      <RegistryOperatorTable 
        data={data?.data || []} 
        isLoading={isLoading} 
      />
    </div>
  );
}
```

### 5. Create Form (`app/registry-operators/create/page.tsx`)

```typescript
'use client';

import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useRouter } from 'next/navigation';
import { useCreateRegistryOperator } from '@/lib/hooks/useRegistryOperators';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { toast } from '@/components/ui/use-toast';

const formSchema = z.object({
  RyID: z.string().min(1, 'RyID is required'),
  Name: z.string().min(1, 'Name is required'),
  Email: z.string().email('Invalid email address'),
  URL: z.string().url('Invalid URL').optional().or(z.literal('')),
  Voice: z.string().optional(),
  Fax: z.string().optional(),
});

type FormValues = z.infer<typeof formSchema>;

export default function CreateRegistryOperatorPage() {
  const router = useRouter();
  const { mutate, isPending } = useCreateRegistryOperator();
  
  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      RyID: '',
      Name: '',
      Email: '',
      URL: '',
      Voice: '',
      Fax: '',
    },
  });

  const onSubmit = (data: FormValues) => {
    mutate(data, {
      onSuccess: () => {
        toast({
          title: 'Success',
          description: 'Registry operator created successfully',
        });
        router.push('/registry-operators');
      },
      onError: (error: any) => {
        toast({
          title: 'Error',
          description: error.response?.data?.error || 'Failed to create registry operator',
          variant: 'destructive',
        });
      },
    });
  };

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Create Registry Operator</h1>
        <p className="text-muted-foreground">
          Add a new registry operator to the system
        </p>
      </div>

      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
          <FormField
            control={form.control}
            name="RyID"
            render={({ field }) => (
              <FormItem>
                <FormLabel>RyID *</FormLabel>
                <FormControl>
                  <Input placeholder="RY-001" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="Name"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Name *</FormLabel>
                <FormControl>
                  <Input placeholder="Registry Name" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="Email"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Email *</FormLabel>
                <FormControl>
                  <Input type="email" placeholder="contact@registry.com" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="URL"
            render={({ field }) => (
              <FormItem>
                <FormLabel>URL</FormLabel>
                <FormControl>
                  <Input placeholder="https://registry.com" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="Voice"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Phone</FormLabel>
                <FormControl>
                  <Input placeholder="+1.1234567890" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="Fax"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Fax</FormLabel>
                <FormControl>
                  <Input placeholder="+1.1234567890" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className="flex gap-4">
            <Button type="submit" disabled={isPending}>
              {isPending ? 'Creating...' : 'Create Registry Operator'}
            </Button>
            <Button 
              type="button" 
              variant="outline" 
              onClick={() => router.back()}
            >
              Cancel
            </Button>
          </div>
        </form>
      </Form>
    </div>
  );
}
```

## Next Steps

### Immediate Actions (Today):

1. **Review this document** - Understand the architecture
2. **Choose learning path**:
   - Option A: Follow official Next.js tutorial first (4 hours)
   - Option B: Jump straight in with the quick start commands
3. **Decision**: Want me to create a complete starter template? (Y/N)

### This Week:
- Set up the Next.js project
- Get API client working
- Build basic layout
- Create first Registry Operator form

### Next Week:
- Complete CRUD operations
- Add mock authentication
- Polish the UI

## Questions to Answer

1. **Do you want a complete starter template?** I can create all the boilerplate for you.

2. **Authentication preference?** 
   - Mock login for now (faster)
   - Set up Auth0 now (more realistic)

3. **Design preference?**
   - Clean/minimal (like Vercel/Linear)
   - Rich/detailed (like Admin dashboards)
   - Custom brand colors?

4. **Deployment target?**
   - Vercel (easiest)
   - Docker container (to match your backend)
   - Other?

5. **Development time available?**
   - Full-time (2 weeks)
   - Part-time (4-6 weeks)
   - Weekend warrior (8-10 weeks)

---

**Let me know how you want to proceed and I'll help you get started!** 🚀
