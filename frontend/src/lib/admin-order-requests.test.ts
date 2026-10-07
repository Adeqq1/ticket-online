import { expect, test } from "bun:test";
import { applyIfCurrent } from "./admin-order-requests.ts";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

test("late order detail responses cannot replace the selected order or a closed detail", async () => {
  let generation = 0;
  let selected = "";
  const requestA = deferred<string>();
  const generationA = ++generation;
  const readA = applyIfCurrent(() => generationA === generation, () => requestA.promise, (value) => { selected = value; });
  const requestB = deferred<string>();
  const generationB = ++generation;
  const readB = applyIfCurrent(() => generationB === generation, () => requestB.promise, (value) => { selected = value; });
  requestB.resolve("order B");
  await readB;
  requestA.resolve("order A");
  await readA;
  expect(selected).toBe("order B");

  const requestClosed = deferred<string>();
  const generationClosed = ++generation;
  const readClosed = applyIfCurrent(() => generationClosed === generation, () => requestClosed.promise, (value) => { selected = value; });
  ++generation;
  requestClosed.resolve("closed order");
  await readClosed;
  expect(selected).toBe("order B");
});

test("refund completion is applied only to the order that started the request", async () => {
  let selectedID = "order A";
  const request = deferred<string>();
  const messages: string[] = [];
  let postCount = 0;
  const orderID = selectedID;
  const submit = applyIfCurrent(() => selectedID === orderID, async () => {
    postCount++;
    return request.promise;
  }, (result) => { messages.push(`${orderID}: ${result}`); });
  selectedID = "order B";
  request.resolve("accepted");
  await submit;
  expect(messages).toEqual([]);
  expect(postCount).toBe(1);
});
