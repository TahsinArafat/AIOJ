# Sub-Plan 21: Mobile PWA

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a Progressive Web App with offline support and mobile optimization.

**Architecture:** Add service worker, manifest file, offline caching, responsive design improvements.

**Tech Stack:** React, TypeScript, Workbox

---

## File Structure

### Frontend Files to Create
- `web/public/manifest.json` - PWA manifest
- `web/public/service-worker.js` - Service worker
- `web/src/hooks/useOnlineStatus.ts` - Online status hook
- `web/src/components/OfflineBanner.tsx` - Offline indicator

### Frontend Files to Modify
- `web/index.html` - Add meta tags
- `web/vite.config.ts` - Add PWA plugin
- `web/src/global.css` - Mobile optimizations

---

## Tasks

### Task 1: PWA Manifest

**Files:**
- Create: `web/public/manifest.json`

- [ ] **Step 1: Create manifest file**

```json
{
  "name": "AIOJ - Online Judge",
  "short_name": "AIOJ",
  "description": "Competitive Programming Platform",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#ffffff",
  "theme_color": "#2563eb",
  "orientation": "any",
  "icons": [
    {
      "src": "/icons/icon-192.png",
      "sizes": "192x192",
      "type": "image/png",
      "purpose": "any maskable"
    },
    {
      "src": "/icons/icon-512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "any maskable"
    }
  ],
  "categories": ["education", "developer"],
  "screenshots": [
    {
      "src": "/screenshots/desktop.png",
      "sizes": "1280x720",
      "type": "image/png",
      "form_factor": "wide"
    },
    {
      "src": "/screenshots/mobile.png",
      "sizes": "750x1334",
      "type": "image/png",
      "form_factor": "narrow"
    }
  ]
}
```

- [ ] **Step 2: Create placeholder icons**

Run:
```bash
mkdir -p web/public/icons web/public/screenshots
# Create placeholder icons (would use real icons in production)
convert -size 192x192 xc:#2563eb -fill white -pointsize 72 -gravity center -annotate 0 "A" web/public/icons/icon-192.png
convert -size 512x512 xc:#2563eb -fill white -pointsize 192 -gravity center -annotate 0 "A" web/public/icons/icon-512.png
```

- [ ] **Step 3: Commit**

```bash
git add web/public/manifest.json web/public/icons/
git commit -m "feat(pwa): add PWA manifest and icons"
```

---

### Task 2: Service Worker

**Files:**
- Create: `web/public/service-worker.js`

- [ ] **Step 1: Create service worker**

```javascript
// web/public/service-worker.js
const CACHE_NAME = 'aioj-v1';
const STATIC_ASSETS = [
  '/',
  '/index.html',
  '/manifest.json',
];

// Install event - cache static assets
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(STATIC_ASSETS);
    })
  );
  self.skipWaiting();
});

// Activate event - clean old caches
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames
          .filter((name) => name !== CACHE_NAME)
          .map((name) => caches.delete(name))
      );
    })
  );
  self.clients.claim();
});

// Fetch event - network first, fallback to cache
self.addEventListener('fetch', (event) => {
  // Skip non-GET requests
  if (event.request.method !== 'GET') return;
  
  // Skip API requests
  if (event.request.url.includes('/api/')) {
    return;
  }
  
  event.respondWith(
    fetch(event.request)
      .then((response) => {
        // Cache successful responses
        if (response.ok) {
          const responseClone = response.clone();
          caches.open(CACHE_NAME).then((cache) => {
            cache.put(event.request, responseClone);
          });
        }
        return response;
      })
      .catch(() => {
        // Return cached response
        return caches.match(event.request).then((response) => {
          if (response) return response;
          
          // Return offline page for navigation
          if (event.request.mode === 'navigate') {
            return caches.match('/offline.html');
          }
          
          return new Response('Offline', { status: 503 });
        });
      })
  );
});
```

- [ ] **Step 2: Create offline page**

```html
<!-- web/public/offline.html -->
<!DOCTYPE html>
<html>
<head>
  <title>AIOJ - Offline</title>
  <style>
    body { font-family: sans-serif; text-align: center; padding: 50px; }
    h1 { color: #2563eb; }
  </style>
</head>
<body>
  <h1>You're Offline</h1>
  <p>Please check your internet connection and try again.</p>
  <button onclick="location.reload()">Retry</button>
</body>
</html>
```

- [ ] **Step 3: Commit**

```bash
git add web/public/service-worker.js web/public/offline.html
git commit -m "feat(pwa): add service worker and offline page"
```

---

### Task 3: Update HTML and Config

**Files:**
- Modify: `web/index.html`
- Modify: `web/vite.config.ts`

- [ ] **Step 1: Add meta tags to index.html**

Add to `<head>`:

```html
<!-- PWA Meta Tags -->
<link rel="manifest" href="/manifest.json" />
<meta name="theme-color" content="#2563eb" />
<meta name="apple-mobile-web-app-capable" content="yes" />
<meta name="apple-mobile-web-app-status-bar-style" content="default" />
<meta name="apple-mobile-web-app-title" content="AIOJ" />
<link rel="apple-touch-icon" href="/icons/icon-192.png" />

<!-- Mobile Meta Tags -->
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=5" />
<meta name="mobile-web-app-capable" content="yes" />

<!-- Open Graph -->
<meta property="og:title" content="AIOJ - Online Judge" />
<meta property="og:description" content="Competitive Programming Platform" />
<meta property="og:type" content="website" />
<meta property="og:url" content="https://aioj.net" />
```

- [ ] **Step 2: Register service worker**

Add to `web/index.html` before `</body>`:

```html
<script>
  if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
      navigator.serviceWorker.register('/service-worker.js')
        .then((reg) => console.log('SW registered:', reg.scope))
        .catch((err) => console.log('SW registration failed:', err));
    });
  }
</script>
```

- [ ] **Step 3: Commit**

```bash
git add web/index.html
git commit -m "feat(pwa): add PWA meta tags and service worker registration"
```

---

### Task 4: Online Status Hook

**Files:**
- Create: `web/src/hooks/useOnlineStatus.ts`
- Create: `web/src/components/OfflineBanner.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Create online status hook**

```typescript
// web/src/hooks/useOnlineStatus.ts
import { useState, useEffect } from 'react';

export function useOnlineStatus() {
  const [isOnline, setIsOnline] = useState(navigator.onLine);

  useEffect(() => {
    const handleOnline = () => setIsOnline(true);
    const handleOffline = () => setIsOnline(false);

    window.addEventListener('online', handleOnline);
    window.addEventListener('offline', handleOffline);

    return () => {
      window.removeEventListener('online', handleOnline);
      window.removeEventListener('offline', handleOffline);
    };
  }, []);

  return isOnline;
}
```

- [ ] **Step 2: Create OfflineBanner component**

```tsx
// web/src/components/OfflineBanner.tsx
import { useOnlineStatus } from '../hooks/useOnlineStatus';

export default function OfflineBanner() {
  const isOnline = useOnlineStatus();

  if (isOnline) return null;

  return (
    <div className="bg-yellow-500 text-white text-center py-2 px-4 text-sm">
      ⚠️ You're offline. Some features may be unavailable.
    </div>
  );
}
```

- [ ] **Step 3: Add to App**

```tsx
// web/src/App.tsx
import OfflineBanner from './components/OfflineBanner';

export default function App() {
  return (
    <BrowserRouter>
      <div className="min-h-screen bg-white">
        <OfflineBanner />
        <Navbar />
        {/* ... */}
      </div>
    </BrowserRouter>
  );
}
```

- [ ] **Step 4: Commit**

```bash
git add web/src/hooks/useOnlineStatus.ts web/src/components/OfflineBanner.tsx web/src/App.tsx
git commit -m "feat(pwa): add online status detection"
```

---

### Task 5: Mobile Responsive Improvements

**Files:**
- Modify: `web/src/global.css`

- [ ] **Step 1: Add mobile optimizations**

```css
/* web/src/global.css additions */

/* Touch-friendly tap targets */
@media (max-width: 768px) {
  button, a, [role="button"] {
    min-height: 44px;
    min-width: 44px;
  }
  
  /* Larger text for readability */
  body {
    font-size: 16px;
  }
  
  /* Stack layout on mobile */
  .grid-cols-2 {
    grid-template-columns: 1fr;
  }
  
  /* Full-width buttons */
  .btn-mobile-full {
    width: 100%;
  }
  
  /* Adjust spacing */
  .px-6 {
    padding-left: 1rem;
    padding-right: 1rem;
  }
}

/* Safe area for notched devices */
@supports (padding: max(0px)) {
  .safe-area-bottom {
    padding-bottom: max(1rem, env(safe-area-inset-bottom));
  }
}

/* Prevent pull-to-refresh */
body {
  overscroll-behavior-y: contain;
}
```

- [ ] **Step 2: Update Navbar for mobile**

```tsx
// Add hamburger menu for mobile
const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

// Mobile menu button
<button
  onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
  className="md:hidden p-2"
>
  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
  </svg>
</button>

// Mobile menu dropdown
{mobileMenuOpen && (
  <div className="md:hidden absolute top-full left-0 right-0 bg-white border-b shadow-lg">
    <div className="px-4 py-2 space-y-2">
      <Link to="/problems" className="block py-2">Problems</Link>
      <Link to="/contests" className="block py-2">Contests</Link>
      {/* ... other links ... */}
    </div>
  </div>
)}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/global.css web/src/App.tsx
git commit -m "feat(pwa): add mobile responsive improvements"
```

---

## Verification Checklist

- [ ] PWA manifest loads correctly
- [ ] Service worker registers
- [ ] Offline page shows when offline
- [ ] App installable on mobile
- [ ] Mobile menu works
- [ ] Touch targets are large enough
- [ ] No horizontal scroll on mobile

---

## Notes

1. **Caching**: Static assets cached, API calls not cached
2. **Offline**: Shows offline banner, cached pages still work
3. **Install**: Add to Home Screen prompt
4. **Mobile**: Touch-friendly, responsive layout
