import { expect, test } from "bun:test";
import { createRequestVersion } from "./request-version.ts";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

test("only the newest case or email detail response can update selection", async () => {
  const requests = createRequestVersion();
  let selected = "";
  let detailError = "";
  const read = async (version: number, response: Promise<string>) => {
    try {
      const result = await response;
      if (requests.isCurrent(version)) { selected = result; detailError = ""; }
    } catch {
      if (requests.isCurrent(version)) detailError = "detail failed";
    }
  };

  const caseA = deferred<string>();
  const emailB = deferred<string>();
  const caseVersion = requests.next();
  const emailVersion = requests.next();
  const oldCaseRead = read(caseVersion, caseA.promise);
  const currentEmailRead = read(emailVersion, emailB.promise);
  emailB.resolve("email B");
  await currentEmailRead;
  caseA.resolve("case A");
  await oldCaseRead;
  expect(selected).toBe("email B");

  const emailA = deferred<string>();
  const caseB = deferred<string>();
  const oldEmailVersion = requests.next();
  const currentCaseVersion = requests.next();
  const oldEmailRead = read(oldEmailVersion, emailA.promise);
  const currentCaseRead = read(currentCaseVersion, caseB.promise);
  caseB.resolve("case B");
  await currentCaseRead;
  emailA.resolve("email A");
  await oldEmailRead;
  expect(selected).toBe("case B");

  const staleFailureVersion = requests.next();
  const currentDetailVersion = requests.next();
  const staleFailure = read(staleFailureVersion, Promise.reject(new Error("old failure")));
  const currentDetail = read(currentDetailVersion, Promise.resolve("payment case"));
  await Promise.all([staleFailure, currentDetail]);
  expect(selected).toBe("payment case");
  expect(detailError).toBe("");
});
