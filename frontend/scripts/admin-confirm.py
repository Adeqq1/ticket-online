"""Browser helpers for styled admin confirmations and native unload prompts."""
from playwright.async_api import expect


async def answer_confirmation(page, accept=True):
    dialog = page.locator(".admin-confirm[open]")
    await expect(dialog).to_be_visible()
    await dialog.get_by_role("button").nth(1 if accept else 0).click()
    await expect(dialog).to_have_count(0)


async def auto_confirm(page):
    # Only fixture flows opt into automatic approval; beforeunload stays native.
    await page.add_init_script("""new MutationObserver(() => {
      const dialog = document.querySelector('.admin-confirm[open]');
      dialog?.querySelector('button:last-child')?.click();
    }).observe(document, {subtree: true, childList: true, attributes: true, attributeFilter: ['open']});""")
