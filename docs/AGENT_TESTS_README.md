# Agent Navigation Feature - Unit Tests

## Quick Start

Run all tests:
```bash
./scripts/run-tests.sh
```

Run backend tests only:
```bash
go test ./internal/agent/service/... -v -cover
```

## Test Files

### Backend (Go) - ✅ Ready & Passing
- `internal/agent/service/agent_service_navigation_test.go`
- **27 test cases** covering all navigation logic
- **100% passing** (0.530s execution time)

### Frontend (TypeScript/React) - ⚠️ Ready (needs framework)
- `frontend/lib/types/__tests__/agent.test.ts` (Type tests)
- `frontend/components/agent/__tests__/agent-chat.test.tsx` (Component tests)
- **52 test cases** covering UI, interactions, and storage
- Requires vitest installation (see setup below)

## Frontend Setup

Install testing dependencies:
```bash
cd frontend
npm install --save-dev vitest @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom
```

Add to `package.json`:
```json
{
  "scripts": {
    "test": "vitest",
    "test:ui": "vitest --ui",
    "test:coverage": "vitest --coverage"
  }
}
```

Create `vitest.config.ts`:
```typescript
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './vitest.setup.ts',
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './'),
    },
  },
})
```

Create `vitest.setup.ts`:
```typescript
import '@testing-library/jest-dom'
```

Run tests:
```bash
npm test
```

## Test Coverage

### Backend Tests (27 cases)
- ✅ TLD navigation (4 tests)
- ✅ Registry operator navigation (3 tests)
- ✅ Domain navigation (4 tests)
- ✅ Dashboard navigation (3 tests)
- ✅ Auto-navigation triggers (5 tests)
- ✅ Case insensitivity (3 tests)
- ✅ No-action scenarios (3 tests)
- ✅ Struct validation (2 tests)

### Frontend Tests (52 cases)
- Component Rendering (5 tests)
- User Interactions (5 tests)
- Message Handling (5 tests)
- Navigation Actions (5 tests)
- LocalStorage Persistence (5 tests)
- Error Handling (3 tests)
- Type Safety (24 tests)

## What's Tested

### ✅ Navigation Actions
- All routes: /tlds, /registry-operators, /domains, /
- Auto-navigation on triggers ("show me", "open", "go to", etc.)
- Manual navigation via buttons
- Multiple navigation options
- Drawer closing on navigation

### ✅ Chat History Persistence
- Save to localStorage on message send
- Load from localStorage on mount
- Clear history functionality
- Corrupt data handling

### ✅ User Interactions
- Text input and validation
- Enter to send, Shift+Enter for new line
- Button enable/disable states
- Loading states during API calls

### ✅ Error Handling
- Network failures
- API errors
- Invalid responses
- Graceful degradation

## Documentation

- **Comprehensive Guide**: [docs/AGENT_NAVIGATION_TESTS.md](../docs/AGENT_NAVIGATION_TESTS.md)
  - Detailed test descriptions
  - Setup instructions
  - CI/CD integration guide
  - Future enhancements

- **Quick Summary**: [docs/TEST_SUMMARY.md](../docs/TEST_SUMMARY.md)
  - Test results overview
  - Coverage metrics
  - Next steps

## Coverage Report

View backend coverage:
```bash
go test ./internal/agent/service/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

View frontend coverage (after setup):
```bash
cd frontend
npm run test:coverage
```

## CI/CD Integration

Example GitHub Actions workflow:
```yaml
name: Test Agent Navigation

on: [push, pull_request]

jobs:
  backend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24.1'
      - run: go test ./internal/agent/service/... -v -cover

  frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '20'
      - run: cd frontend && npm ci && npm test
```

## Test Results

Latest run:
```
✅ Backend:  27/27 tests passing (0.530s)
⏳ Frontend: 52 tests ready (needs framework installation)
📊 Coverage: 13.9% of agent service statements
```

## Contributing

When adding new features:
1. Write tests first (TDD)
2. Run tests locally before committing
3. Ensure all tests pass
4. Update documentation if needed

## Support

For questions or issues:
- See detailed docs in `docs/AGENT_NAVIGATION_TESTS.md`
- Check test examples in test files
- Review test output for failure details
