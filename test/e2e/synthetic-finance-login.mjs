import assert from 'node:assert/strict';

// Dedicated fictional test accounts only. Never reuse the owner's browser profile.
export async function syntheticFinanceLogin(context, page, base) {
  if (process.env.FINANCE_SYNTHETIC_ONLY !== '1' || !['localhost', '127.0.0.1'].includes(new URL(base).hostname)) throw Error('Explicit loopback synthetic fixture required');
  if (process.env.FINANCE_MANAGED === '1') {
    await page.goto(new URL(base).origin + '/_workspace/open/finance');
    await Promise.race([
      page.locator('#username').waitFor({timeout: 60000}),
      page.waitForURL(url => url.pathname.startsWith('/finance/'), {timeout: 60000})
    ]);
    if (await page.locator('#username').isVisible()) {
      await page.locator('#username').fill('owner@example.test');
      await page.getByRole('button', {name:'Next', exact:true}).click();
      await page.locator('input[type=password]').fill('Fictional-Test-Only-582!');
      await page.getByRole('button', {name:/Sign in/i}).click();
    }
    for (let i=0; i<60; i++) {
      const me = await context.request.get(base+'api/me');
      if (me.status()===200 && (await me.json()).user?.id) return;
      await page.waitForTimeout(500);
    }
    throw Error('Managed fictional OneAccounts login did not reach Finance');
  }
  const login = await context.request.post(base+'api/login', {data:{email:'owner@example.test',password:'Fictional-Test-Only-582!'},headers:{Origin:new URL(base).origin}});
  assert.equal(login.status(), 200, 'synthetic account login');
}
