# Registrar Management - Quick Reference

## 🚀 Quick Start

### Access the Interface

```
Navigate to: http://localhost:3000/registrars
```

### Tabs

1. **IANA Registrars** - Official IANA registry (synced from IANA)
2. **System Registrars** - Internal registry registrars

## 📁 File Structure

```
frontend/
├── lib/
│   ├── types/registrar.ts         # Type definitions
│   ├── api/registrars.ts          # API client
│   └── hooks/useRegistrars.ts     # React Query hooks
├── components/registrars/
│   ├── iana-registrars-tab.tsx    # IANA UI
│   └── system-registrars-tab.tsx  # System UI
└── app/registrars/page.tsx        # Main page
```

## 🔌 API Endpoints

### IANA Registrars

```typescript
GET  /ianaregistrars?pagesize=50&name_like=example&status=Accredited
GET  /ianaregistrars/:gurID
GET  /ianaregistrars/count
PUT  /sync/iana-registrars
```

### System Registrars

```typescript
GET    /registrars?pagesize=50
GET    /registrars/:clid
GET    /registrars/gurid/:gurid
GET    /registrars/count
POST   /registrars
PUT    /registrars/:clid
PUT    /registrars/:clid/status/:status
DELETE /registrars/:clid
POST   /registrars/bulk
```

## 🪝 React Query Hooks

### IANA Registrars

```typescript
// List with filters
const { data, isLoading, error } = useIANARegistrars({
  pagesize: 50,
  name_like: "example",
  status: "Accredited"
});

// Get by IANA ID
const { data } = useIANARegistrar(123);

// Count
const { data } = useIANARegistrarCount();

// Sync
const syncMutation = useSyncIANARegistrars();
await syncMutation.mutateAsync();
```

### System Registrars

```typescript
// List
const { data } = useRegistrars({ pagesize: 50 });

// Get by ClID
const { data } = useRegistrar("my-registrar-007");

// Get by GurID
const { data } = useRegistrarByGurID(123);

// Count
const { data } = useRegistrarCount();

// Create
const createMutation = useCreateRegistrar();
await createMutation.mutateAsync({ /* registrar data */ });

// Update
const updateMutation = useUpdateRegistrar();
await updateMutation.mutateAsync({ 
  clid: "my-registrar-007", 
  data: { /* updates */ } 
});

// Delete
const deleteMutation = useDeleteRegistrar();
await deleteMutation.mutateAsync("my-registrar-007");
```

## 📊 Type Definitions

### IANA Registrar

```typescript
interface IANARegistrar {
  GurID: number;
  Name: string;
  Status: "Accredited" | "Terminated" | "Reserved" | "Unknown";
  RdapURL: string;
  CreatedAt: string;
}
```

### System Registrar

```typescript
interface RegistrarListItem {
  ClID: string;
  Name: string;
  GurID: number;
  Status: "ok" | "readonly" | "terminated";
  Autorenew: boolean;
}
```

## 🎨 UI Features

### IANA Registrars Tab

- ✅ Search by name or IANA ID
- ✅ Filter by status (Accredited, Terminated, Reserved, Unknown)
- ✅ Sync from IANA button
- ✅ Total count display
- ✅ Last sync timestamp
- ✅ Status badges
- ✅ RDAP URL links
- ✅ Loading/error states

### System Registrars Tab

- ✅ View all system registrars
- ✅ Display ClID, Name, IANA ID, Status, Auto-renew
- ✅ Status badges
- ✅ Total count
- ⏳ CRUD operations (hooks ready, UI pending)

## 🔧 Configuration

Add to `.env.local`:

```bash
NEXT_PUBLIC_API_BASE_URL=http://localhost:8000
```

## 🧪 Testing

```bash
# Backend API tests
curl http://localhost:8000/ianaregistrars
curl http://localhost:8000/ianaregistrars/count
curl -X PUT http://localhost:8000/sync/iana-registrars

# Frontend
npm run dev
# Navigate to http://localhost:3000/registrars
```

## 📝 Common Tasks

### Add New Filter

1. Update `IANARegistrarListParams` in `lib/types/registrar.ts`
2. Add state in `iana-registrars-tab.tsx`
3. Add to `queryParams` object
4. Add UI control (Select, Input, etc.)

### Add New API Function

1. Define types in `lib/types/registrar.ts`
2. Add function in `lib/api/registrars.ts`
3. Create hook in `lib/hooks/useRegistrars.ts`
4. Use in component

### Handle Sync Errors

```typescript
const syncMutation = useSyncIANARegistrars();

try {
  await syncMutation.mutateAsync();
  toast.success("Sync successful");
} catch (error) {
  toast.error(error.message);
}
```

## 🚨 Troubleshooting

**TypeScript errors:**
```bash
# Restart TS server
npx tsc --noEmit
```

**API connection issues:**
```bash
# Check env var
echo $NEXT_PUBLIC_API_BASE_URL

# Verify backend is running
curl http://localhost:8000/health
```

**Toast notifications not showing:**
```typescript
// Ensure Toaster is in layout
import { Toaster } from "@/components/ui/sonner";

export default function RootLayout({ children }) {
  return (
    <html>
      <body>
        {children}
        <Toaster />
      </body>
    </html>
  );
}
```

## 📚 Resources

- [IANA Registrar IDs](https://www.iana.org/assignments/registrar-ids/registrar-ids.xhtml)
- [React Query Docs](https://tanstack.com/query/latest/docs/framework/react/overview)
- [Shadcn/UI Components](https://ui.shadcn.com/docs/components)
- [Backend API Docs](http://localhost:8000/swagger/index.html)

## ✅ Status

- **IANA Registrars:** ✅ Complete
- **System Registrars:** ⏳ Read-only (CRUD UI pending)
- **Pagination:** ⏳ Backend ready, UI pending
- **Documentation:** ✅ Complete
