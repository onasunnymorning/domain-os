# Registrar Management - Architecture Diagram

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         USER INTERFACE                          │
│                    (http://localhost:3000)                      │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 │
        ┌────────────────────────┴────────────────────────┐
        │                                                  │
        ▼                                                  ▼
┌──────────────────┐                            ┌──────────────────┐
│  IANA Registrars │                            │ System Registrars│
│       Tab        │                            │       Tab        │
└──────────────────┘                            └──────────────────┘
        │                                                  │
        │                                                  │
        └────────────────────────┬────────────────────────┘
                                 │
                                 ▼
                    ┌─────────────────────────┐
                    │   React Query Hooks     │
                    │  (State Management)     │
                    └─────────────────────────┘
                                 │
                                 │
                    ┌────────────┴────────────┐
                    │                         │
                    ▼                         ▼
          ┌─────────────────┐      ┌─────────────────┐
          │ IANA Registrar  │      │ System Registrar│
          │     Hooks       │      │     Hooks       │
          └─────────────────┘      └─────────────────┘
                    │                         │
                    │                         │
                    └────────────┬────────────┘
                                 │
                                 ▼
                    ┌─────────────────────────┐
                    │      API Client         │
                    │   (lib/api/registrars)  │
                    └─────────────────────────┘
                                 │
                                 │ HTTP/REST
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                      BACKEND API SERVER                         │
│                    (http://localhost:8000)                      │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 │
        ┌────────────────────────┴────────────────────────┐
        │                                                  │
        ▼                                                  ▼
┌──────────────────┐                            ┌──────────────────┐
│ IANA Registrar   │                            │ System Registrar │
│   Controller     │                            │   Controller     │
└──────────────────┘                            └──────────────────┘
        │                                                  │
        │                                                  │
        ▼                                                  ▼
┌──────────────────┐                            ┌──────────────────┐
│ IANA Registrar   │                            │ System Registrar │
│    Service       │                            │    Service       │
└──────────────────┘                            └──────────────────┘
        │                                                  │
        │                                                  │
        └────────────────────────┬────────────────────────┘
                                 │
                                 ▼
                    ┌─────────────────────────┐
                    │   PostgreSQL Database   │
                    │   (Registrar Storage)   │
                    └─────────────────────────┘
```

## Component Hierarchy

```
app/registrars/page.tsx
│
├── Tabs (Shadcn UI)
│   │
│   ├── TabsList
│   │   ├── TabsTrigger: "IANA Registrars"
│   │   └── TabsTrigger: "System Registrars"
│   │
│   ├── TabsContent: IANA
│   │   └── IANARegistrarsTab
│   │       │
│   │       ├── Info Card
│   │       │   ├── Total count
│   │       │   ├── Last sync timestamp
│   │       │   └── Sync button
│   │       │
│   │       ├── Filters Card
│   │       │   ├── Search input
│   │       │   └── Status select
│   │       │
│   │       └── Results Card
│   │           └── Table
│   │               ├── IANA ID
│   │               ├── Name
│   │               ├── Status (badge)
│   │               └── RDAP URL (link)
│   │
│   └── TabsContent: System
│       └── SystemRegistrarsTab
│           │
│           ├── Info Card
│           │   └── Total count
│           │
│           ├── Filters Card (disabled)
│           │   └── Search input
│           │
│           └── Results Card
│               └── Table
│                   ├── ClID
│                   ├── Name
│                   ├── IANA ID
│                   ├── Status (badge)
│                   └── Auto-renew (badge)
```

## Data Flow

### IANA Registrar List

```
┌─────────────┐
│    User     │
│   Action    │
└──────┬──────┘
       │
       │ 1. Enters search/filter
       ▼
┌─────────────────────┐
│ IANARegistrarsTab   │
│  Component          │
└──────┬──────────────┘
       │
       │ 2. Updates state
       ▼
┌─────────────────────┐
│ useIANARegistrars   │
│    Hook             │
└──────┬──────────────┘
       │
       │ 3. Calls API with params
       ▼
┌─────────────────────┐
│ getIANARegistrars   │
│   API Function      │
└──────┬──────────────┘
       │
       │ 4. GET /ianaregistrars?params
       ▼
┌─────────────────────┐
│  Backend API        │
│  Controller         │
└──────┬──────────────┘
       │
       │ 5. Service.List()
       ▼
┌─────────────────────┐
│    Database         │
│    Query            │
└──────┬──────────────┘
       │
       │ 6. Returns data
       │
       ▼
┌─────────────────────┐
│  React Query        │
│  Cache              │
└──────┬──────────────┘
       │
       │ 7. Updates component
       ▼
┌─────────────────────┐
│   Table Renders     │
│   with Data         │
└─────────────────────┘
```

### Sync IANA Registrars

```
┌─────────────┐
│    User     │
│   Clicks    │
│   "Sync"    │
└──────┬──────┘
       │
       │ 1. onClick handler
       ▼
┌─────────────────────┐
│  handleSync()       │
│  Function           │
└──────┬──────────────┘
       │
       │ 2. Triggers mutation
       ▼
┌─────────────────────┐
│ useSyncIANARegistrars│
│    Hook             │
└──────┬──────────────┘
       │
       │ 3. Calls API
       ▼
┌─────────────────────┐
│ syncIANARegistrars  │
│   API Function      │
└──────┬──────────────┘
       │
       │ 4. PUT /sync/iana-registrars
       ▼
┌─────────────────────┐
│  Backend Sync       │
│  Controller         │
└──────┬──────────────┘
       │
       │ 5. Downloads IANA XML
       │    Parses and stores
       ▼
┌─────────────────────┐
│    Database         │
│    Updated          │
└──────┬──────────────┘
       │
       │ 6. Success response
       │
       ▼
┌─────────────────────┐
│  Mutation Success   │
│  Handler            │
└──────┬──────────────┘
       │
       │ 7. Invalidates cache
       │    Shows toast
       │
       ▼
┌─────────────────────┐
│  Data Refetches     │
│  Table Updates      │
└─────────────────────┘
```

## File Dependencies

```
frontend/app/registrars/page.tsx
│
├── imports @/components/ui/tabs
├── imports @/components/registrars/iana-registrars-tab
│   │
│   ├── imports @/lib/hooks/useRegistrars
│   │   │
│   │   ├── imports @tanstack/react-query
│   │   └── imports @/lib/api/registrars
│   │       │
│   │       └── imports @/lib/types/registrar
│   │
│   ├── imports @/components/ui/button
│   ├── imports @/components/ui/input
│   ├── imports @/components/ui/select
│   ├── imports @/components/ui/card
│   ├── imports @/components/ui/table
│   ├── imports @/components/ui/badge
│   ├── imports lucide-react
│   └── imports sonner
│
└── imports @/components/registrars/system-registrars-tab
    │
    ├── imports @/lib/hooks/useRegistrars
    ├── imports @/components/ui/card
    ├── imports @/components/ui/table
    ├── imports @/components/ui/badge
    ├── imports @/components/ui/input
    └── imports lucide-react
```

## API Endpoint Mapping

```
Frontend Hook              →  API Function            →  Backend Endpoint
─────────────────────────────────────────────────────────────────────────

IANA Registrars:
useIANARegistrars()       →  getIANARegistrars()     →  GET /ianaregistrars
useIANARegistrar(id)      →  getIANARegistrarByGurID →  GET /ianaregistrars/:id
useIANARegistrarCount()   →  getIANARegistrarCount() →  GET /ianaregistrars/count
useSyncIANARegistrars()   →  syncIANARegistrars()    →  PUT /sync/iana-registrars

System Registrars:
useRegistrars()           →  getRegistrars()         →  GET /registrars
useRegistrar(clid)        →  getRegistrarByClID()    →  GET /registrars/:clid
useRegistrarByGurID(id)   →  getRegistrarByGurID()   →  GET /registrars/gurid/:id
useRegistrarCount()       →  getRegistrarCount()     →  GET /registrars/count
useCreateRegistrar()      →  createRegistrar()       →  POST /registrars
useUpdateRegistrar()      →  updateRegistrar()       →  PUT /registrars/:clid
useUpdateRegistrarStatus()→  updateRegistrarStatus() →  PUT /registrars/:clid/status/:status
useDeleteRegistrar()      →  deleteRegistrar()       →  DELETE /registrars/:clid
useBulkCreateRegistrars() →  bulkCreateRegistrars()  →  POST /registrars/bulk
```

## State Management Flow

```
┌─────────────────────────────────────────────────────────┐
│                    Component State                      │
│  (useState for UI-only state like search input)         │
└────────────────┬────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────┐
│                   React Query State                     │
│  (Server state: loading, data, error, mutations)        │
└────────────────┬────────────────────────────────────────┘
                 │
                 ├─► Query Keys: ["ianaRegistrars", params]
                 │
                 ├─► Cache: 5-10 min stale time
                 │
                 ├─► Refetch: on window focus, mutation success
                 │
                 └─► Mutations: optimistic updates, invalidation
```

## Error Handling Flow

```
API Error
   │
   ├─► Network Error
   │   └─► Show error toast
   │       └─► Display retry button
   │
   ├─► 404 Not Found
   │   └─► Show "Not found" message
   │
   ├─► 400 Bad Request
   │   └─► Show validation error
   │
   ├─► 500 Server Error
   │   └─► Show generic error
   │       └─► Log to console
   │
   └─► Unknown Error
       └─► Show fallback message
```

## Caching Strategy

```
Query Type         Stale Time    Refetch On       Cache Key Pattern
────────────────────────────────────────────────────────────────────
List queries       5 minutes     Window focus     ["ianaRegistrars", {...params}]
                                 Mutation         ["registrars", {...params}]

Single item        10 minutes    Window focus     ["ianaRegistrar", id]
                                                  ["registrar", clid]

Count queries      5 minutes     Window focus     ["ianaRegistrars", "count"]
                                 Mutation         ["registrars", "count"]

Mutations          -             On success       Invalidates related queries
```

## Performance Optimizations

1. **React Query Caching**
   - Reduces unnecessary API calls
   - Stale-while-revalidate strategy
   - Background refetching

2. **Component Optimization**
   - Client-side rendering for data
   - Lazy loading (future: pagination)
   - Memoization of expensive calculations

3. **Network Optimization**
   - Query parameter filtering on backend
   - Pagination support (cursor-based)
   - Minimal data transfer

4. **UI Optimization**
   - Loading skeletons
   - Optimistic updates
   - Debounced search (future)

---

**This diagram represents the complete architecture of the Registrar Management system.**
