'use strict';
// Notification delivery only. Never intercept requests or cache private content.
self.addEventListener('push', event => {
  event.waitUntil(self.registration.showNotification('My Finance', {
    body: 'Open My Finance to review your crypto monitor.',
    icon: new URL('icon.svg', self.registration.scope).href,
    tag: 'finance-crypto-review',
    renotify: false,
    data: { kind: 'crypto-review' }
  }));
});
self.addEventListener('notificationclick', event => {
  event.notification.close();
  const target = new URL('./#crypto', self.registration.scope).href;
  event.waitUntil((async () => {
    const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
    for (const client of windows) {
      const u = new URL(client.url), scope = new URL(self.registration.scope);
      if (u.origin === scope.origin && u.pathname === scope.pathname) {
        await client.navigate(target);
        return client.focus();
      }
    }
    return self.clients.openWindow(target);
  })());
});
