# EnvHub Frontend Design Document

**Date:** 2026-01-28
**Status:** Draft
**Author:** AI Assistant

## Overview

This document outlines the design for EnvHub's frontend management interface, which provides CRUD operations for Environments, Instances, and Services.

## Technology Stack

### Recommended Stack

- **Framework:** React 18+ with TypeScript
- **Styling:** Tailwind CSS
- **UI Components:** shadcn/ui (or Ant Design as alternative)
- **Routing:** React Router v6
- **State Management:** React Query (TanStack Query) for server state
- **HTTP Client:** Axios
- **Build Tool:** Vite
- **Form Handling:** React Hook Form + Zod validation

### Alternative Options

- **Ant Design Pro:** Enterprise-ready solution with built-in layouts and components
- **Vue 3 + Element Plus:** If team prefers Vue ecosystem

## Architecture

### Directory Structure

```
envhub-frontend/
├── src/
│   ├── api/              # API client and endpoints
│   │   ├── client.ts     # Axios instance with interceptors
│   │   ├── env.ts        # Environment API
│   │   ├── instance.ts   # Instance API
│   │   └── service.ts    # Service API
│   ├── components/       # Reusable components
│   │   ├── ui/           # shadcn/ui components
│   │   ├── Layout/       # Layout components
│   │   ├── EnvCard/      # Environment card component
│   │   ├── StatusBadge/  # Status indicator component
│   │   └── DataTable/    # Reusable table component
│   ├── pages/            # Page components
│   │   ├── Environments/ # Environment management
│   │   ├── Instances/    # Instance management
│   │   └── Services/     # Service management
│   ├── hooks/            # Custom React hooks
│   │   ├── useEnv.ts     # Environment operations
│   │   ├── useInstance.ts # Instance operations
│   │   └── useService.ts  # Service operations
│   ├── types/            # TypeScript type definitions
│   │   ├── env.ts
│   │   ├── instance.ts
│   │   └── service.ts
│   ├── utils/            # Utility functions
│   ├── App.tsx           # Root component
│   └── main.tsx          # Entry point
├── public/
├── index.html
├── package.json
├── tsconfig.json
├── tailwind.config.js
└── vite.config.ts
```

## API Integration

### Base Configuration

```typescript
// src/api/client.ts
import axios from 'axios';

const apiClient = axios.create({
  baseURL: process.env.VITE_API_BASE_URL || 'http://localhost:8080',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor for auth token
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response interceptor for error handling
apiClient.interceptors.response.use(
  (response) => response.data,
  (error) => {
    // Handle common errors
    return Promise.reject(error);
  }
);
```

### API Endpoints

#### Environment API

```typescript
// src/api/env.ts

export interface Environment {
  id: string;
  name: string;
  description: string;
  version: string;
  tags: string[];
  code_url: string;
  status: EnvStatus;
  artifacts: Artifact[];
  build_config: Record<string, any>;
  test_config: Record<string, any>;
  deploy_config: Record<string, any>;
  created_at: string;
  updated_at: string;
}

export enum EnvStatus {
  Init = 0,
  Pending = 1,
  Creating = 2,
  Created = 3,
  Testing = 4,
  Verified = 5,
  Ready = 6,
  Released = 7,
  Failed = 8,
}

export const envApi = {
  // GET /env/
  list: () => apiClient.get<Environment[]>('/env/'),

  // GET /env/:name/:version
  get: (name: string, version: string) =>
    apiClient.get<Environment>(`/env/${name}/${version}`),

  // POST /env/
  create: (data: Partial<Environment>) =>
    apiClient.post<boolean>('/env/', data),

  // PUT /env/:name/:version
  update: (name: string, version: string, data: Partial<Environment>) =>
    apiClient.put<boolean>(`/env/${name}/${version}`, data),

  // POST /env/:name/:version/release
  release: (name: string, version: string) =>
    apiClient.post<boolean>(`/env/${name}/${version}/release`),

  // GET /env/:name/:version/status
  getStatus: (name: string, version: string) =>
    apiClient.get<{status: string}>(`/env/${name}/${version}/status`),

  // GET /env/:name/:version/exists
  exists: (name: string, version: string) =>
    apiClient.get<{exists: boolean, status?: EnvStatus}>(`/env/${name}/${version}/exists`),
};
```

#### Instance API

```typescript
// src/api/instance.ts

export interface EnvInstance {
  id: string;
  name: string;
  env: Environment;
  status: string;
  owner: string;
  created_at: string;
  endpoint?: string;
}

export const instanceApi = {
  // POST /env-instance/
  create: (data: {
    envName: string;
    datasource?: string;
    environment_variables?: Record<string, string>;
    arguments?: string[];
    ttl?: string;
    owner?: string;
  }) => apiClient.post<EnvInstance>('/env-instance/', data),

  // GET /env-instance/:id
  get: (id: string) =>
    apiClient.get<EnvInstance>(`/env-instance/${id}`),

  // DELETE /env-instance/:id
  delete: (id: string) =>
    apiClient.delete<string>(`/env-instance/${id}`),

  // GET /env-instance/:id/list (id can be * for all)
  list: (envName?: string) =>
    apiClient.get<EnvInstance[]>(`/env-instance/${envName || '*'}/list`),

  // POST /env-instance/:id/warmup
  warmup: (id: string) =>
    apiClient.post<Environment>(`/env-instance/${id}/warmup`),
};
```

#### Service API

```typescript
// src/api/service.ts

export interface EnvService {
  id: string;
  name: string;
  env: Environment;
  replicas: number;
  status: string;
  endpoint?: string;
  created_at: string;
}

export const serviceApi = {
  // POST /env-service/
  create: (data: {
    envName: string;
    service_name?: string;
    replicas?: number;
    environment_variables?: Record<string, string>;
    owner?: string;
    pvc_name?: string;
    mount_path?: string;
    storage_size?: string;
    port?: number;
    cpu_request?: string;
    cpu_limit?: string;
    memory_request?: string;
    memory_limit?: string;
    ephemeral_storage_request?: string;
    ephemeral_storage_limit?: string;
  }) => apiClient.post<EnvService>('/env-service/', data),

  // GET /env-service/:id
  get: (id: string) =>
    apiClient.get<EnvService>(`/env-service/${id}`),

  // PUT /env-service/:id
  update: (id: string, data: {
    replicas?: number;
    image?: string;
    environment_variables?: Record<string, string>;
  }) => apiClient.put<EnvService>(`/env-service/${id}`, data),

  // DELETE /env-service/:id?deleteStorage=true
  delete: (id: string, deleteStorage: boolean = false) =>
    apiClient.delete<string>(`/env-service/${id}?deleteStorage=${deleteStorage}`),

  // GET /env-service/:id/list (id can be * for all)
  list: (envName?: string) =>
    apiClient.get<EnvService[]>(`/env-service/${envName || '*'}/list`),
};
```

## Page Designs

### 1. Layout Component

```tsx
// src/components/Layout/MainLayout.tsx
import { Outlet, Link } from 'react-router-dom';

export function MainLayout() {
  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white shadow-sm">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16 items-center">
            <h1 className="text-xl font-bold">EnvHub</h1>
            <nav className="flex gap-6">
              <Link to="/environments" className="hover:text-blue-600">
                Environments
              </Link>
              <Link to="/instances" className="hover:text-blue-600">
                Instances
              </Link>
              <Link to="/services" className="hover:text-blue-600">
                Services
              </Link>
            </nav>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Outlet />
      </main>
    </div>
  );
}
```

### 2. Environments Page

**Features:**

- List all environments with pagination
- Filter by name, version, status, tags
- Create new environment
- Edit environment (if not released)
- Release environment
- View environment details

**Layout:**

- Top bar: Search, filters, "Create Environment" button
- Table/Grid view toggle
- Table columns: Name, Version, Status, Tags, Created At, Actions
- Actions: View, Edit, Release, Delete (conditional based on status)

### 3. Instances Page

**Features:**

- List all instances
- Filter by environment name, owner, status
- Create new instance
- Delete instance
- Warmup instance
- View instance details and logs

**Layout:**

- Top bar: Search, filters, "Create Instance" button
- Table columns: ID, Environment, Status, Owner, Endpoint, Created At, Actions
- Actions: View, Delete, Warmup

### 4. Services Page

**Features:**

- List all services
- Filter by environment name, status
- Create new service
- Update service (replicas, image, env vars)
- Delete service (with option to delete storage)
- View service details

**Layout:**

- Top bar: Search, filters, "Create Service" button
- Table columns: Name, Environment, Replicas, Status, Endpoint, Created At, Actions
- Actions: View, Edit, Scale, Delete

## Component Specifications

### StatusBadge Component

```tsx
// src/components/StatusBadge/StatusBadge.tsx
interface StatusBadgeProps {
  status: EnvStatus | string;
}

const statusColors = {
  Init: 'gray',
  Pending: 'yellow',
  Creating: 'blue',
  Created: 'blue',
  Testing: 'purple',
  Verified: 'green',
  Ready: 'green',
  Released: 'green',
  Failed: 'red',
};

export function StatusBadge({ status }: StatusBadgeProps) {
  const statusName = typeof status === 'number'
    ? EnvStatus[status]
    : status;
  const color = statusColors[statusName] || 'gray';

  return (
    <span className={`badge badge-${color}`}>
      {statusName}
    </span>
  );
}
```

### DataTable Component

Reusable table component with:

- Sorting
- Pagination
- Row selection
- Custom column renderers
- Loading and error states

### Modal Components

- CreateEnvironmentModal
- EditEnvironmentModal
- CreateInstanceModal
- CreateServiceModal
- EditServiceModal
- ConfirmDeleteModal

## State Management

### React Query for Server State

```tsx
// src/hooks/useEnv.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { envApi } from '@/api/env';

export function useEnvironments() {
  return useQuery({
    queryKey: ['environments'],
    queryFn: envApi.list,
  });
}

export function useEnvironment(name: string, version: string) {
  return useQuery({
    queryKey: ['environment', name, version],
    queryFn: () => envApi.get(name, version),
    enabled: !!name && !!version,
  });
}

export function useCreateEnvironment() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: envApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['environments'] });
    },
  });
}

export function useUpdateEnvironment() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, version, data }: any) =>
      envApi.update(name, version, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['environments'] });
    },
  });
}

export function useReleaseEnvironment() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, version }: any) =>
      envApi.release(name, version),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['environments'] });
    },
  });
}
```

Similar patterns for instances and services.

## Routing

```tsx
// src/App.tsx
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MainLayout } from './components/Layout/MainLayout';
import { EnvironmentsPage } from './pages/Environments';
import { InstancesPage } from './pages/Instances';
import { ServicesPage } from './pages/Services';

const queryClient = new QueryClient();

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<MainLayout />}>
            <Route index element={<Navigate to="/environments" replace />} />
            <Route path="environments" element={<EnvironmentsPage />} />
            <Route path="instances" element={<InstancesPage />} />
            <Route path="services" element={<ServicesPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
```

## Form Validation

Using React Hook Form + Zod:

```tsx
// src/types/schemas.ts
import { z } from 'zod';

export const createEnvironmentSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  version: z.string().min(1, 'Version is required'),
  code_url: z.string().url('Must be a valid URL').optional(),
  tags: z.array(z.string()).optional(),
  description: z.string().optional(),
  buildConfig: z.record(z.any()).optional(),
  testConfig: z.record(z.any()).optional(),
  deployConfig: z.record(z.any()).optional(),
});

export const createInstanceSchema = z.object({
  envName: z.string().min(1, 'Environment name is required'),
  datasource: z.string().optional(),
  ttl: z.string().optional(),
  owner: z.string().optional(),
  environment_variables: z.record(z.string()).optional(),
  arguments: z.array(z.string()).optional(),
});

export const createServiceSchema = z.object({
  envName: z.string().min(1, 'Environment name is required'),
  service_name: z.string().optional(),
  replicas: z.number().int().positive().default(1),
  port: z.number().int().positive().optional(),
  owner: z.string().optional(),
  environment_variables: z.record(z.string()).optional(),
  // Resource limits
  cpu_request: z.string().optional(),
  cpu_limit: z.string().optional(),
  memory_request: z.string().optional(),
  memory_limit: z.string().optional(),
  // Storage
  pvc_name: z.string().optional(),
  mount_path: z.string().optional(),
  storage_size: z.string().optional(),
});
```

## Error Handling

```tsx
// src/utils/error.ts
export function getErrorMessage(error: any): string {
  if (error.response?.data?.message) {
    return error.response.data.message;
  }
  if (error.message) {
    return error.message;
  }
  return 'An unexpected error occurred';
}

// Usage in components
const { mutate, isError, error } = useCreateEnvironment();

if (isError) {
  toast.error(getErrorMessage(error));
}
```

## Authentication (Future)

Currently the API may use token-based auth. The frontend should:

1. Store token in localStorage
2. Add token to all requests via axios interceptor
3. Handle 401/403 errors by redirecting to login
4. Add a login page if needed

## Deployment

### Environment Variables

```env
# .env.production
VITE_API_BASE_URL=https://api.envhub.example.com
```

### Build Commands

```bash
# Install dependencies
npm install

# Development
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview
```

### Docker Deployment

```dockerfile
# Dockerfile
FROM node:18-alpine as builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

## Testing Strategy

1. **Unit Tests:** Component logic using Vitest + React Testing Library
2. **Integration Tests:** API integration tests with MSW (Mock Service Worker)
3. **E2E Tests:** Critical user flows with Playwright

## Future Enhancements

1. **Real-time Updates:** WebSocket support for live status updates
2. **Metrics Dashboard:** Visualize resource usage, request rates
3. **Logs Viewer:** Stream and search logs from instances/services
4. **RBAC:** Role-based access control
5. **Audit Log:** Track all CRUD operations
6. **Batch Operations:** Select multiple items for bulk actions
7. **Export/Import:** Export configurations as YAML/JSON

## Implementation Priority

### Phase 1: Core CRUD (Week 1-2)

- [ ] Project setup with Vite + React + TypeScript
- [ ] API client configuration
- [ ] Layout and navigation
- [ ] Environments list and create
- [ ] Instances list and create
- [ ] Services list and create

### Phase 2: Advanced Features (Week 3)

- [ ] Edit/Update operations
- [ ] Delete operations with confirmations
- [ ] Filters and search
- [ ] Status badges and indicators
- [ ] Form validation

### Phase 3: UX Improvements (Week 4)

- [ ] Loading states and skeletons
- [ ] Error handling and toast notifications
- [ ] Responsive design
- [ ] Keyboard shortcuts
- [ ] Dark mode support

### Phase 4: Polish (Week 5)

- [ ] Testing
- [ ] Documentation
- [ ] Performance optimization
- [ ] Accessibility improvements
- [ ] Deployment setup

## Conclusion

This design provides a solid foundation for the EnvHub frontend. The architecture is scalable, maintainable, and follows modern React best practices. The modular structure allows for easy feature additions and modifications.
