import assert from "node:assert/strict";
import test from "node:test";

test("connection status follows real requests and browser connectivity", async (t) => {
  const navigatorDescriptor = Object.getOwnPropertyDescriptor(globalThis, "navigator");
  const originalFetch = globalThis.fetch;
  const originalWindow = globalThis.window;
  const network = { onLine: true };
  Object.defineProperty(globalThis, "navigator", { value: network, configurable: true });
  globalThis.window = new EventTarget();
  t.after(() => {
    Object.defineProperty(globalThis, "navigator", navigatorDescriptor);
    globalThis.fetch = originalFetch;
    globalThis.window = originalWindow;
  });

  const { connectionAvailable, fetchWithConnectionStatus, subscribeToConnection } =
    await import("../src/lib/connection.ts");
  let notifications = 0;
  const unsubscribe = subscribeToConnection(() => notifications++);

  await t.test("detects failed requests even when the browser reports online", async () => {
    globalThis.fetch = async () => { throw new TypeError("Failed to fetch"); };
    await assert.rejects(fetchWithConnectionStatus("/api/v1/accounts"), TypeError);
    assert.equal(network.onLine, true);
    assert.equal(connectionAvailable(), false);
    assert.ok(notifications > 0);
  });

  await t.test("a response restores connectivity even when sign-in is required", async () => {
    const response = new Response("", { status: 401 });
    globalThis.fetch = async () => response;
    assert.equal(await fetchWithConnectionStatus("/api/v1/auth/me"), response);
    assert.equal(connectionAvailable(), true);
  });

  await t.test("canceling a request does not report a disconnected network", async () => {
    globalThis.fetch = async () => { throw new DOMException("Canceled", "AbortError"); };
    await assert.rejects(fetchWithConnectionStatus("/api/v1/accounts"), { name: "AbortError" });
    assert.equal(connectionAvailable(), true);
  });

  await t.test("browser offline and online events update the notice", () => {
    network.onLine = false;
    window.dispatchEvent(new Event("offline"));
    assert.equal(connectionAvailable(), false);
    network.onLine = true;
    window.dispatchEvent(new Event("online"));
    assert.equal(connectionAvailable(), true);
  });

  await t.test("unmounted subscribers stop receiving notifications", () => {
    unsubscribe();
    const previous = notifications;
    network.onLine = false;
    window.dispatchEvent(new Event("offline"));
    assert.equal(notifications, previous);
  });
});
