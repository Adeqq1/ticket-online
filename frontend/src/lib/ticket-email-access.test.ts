import { expect, test } from "bun:test";
import { emailAccessToken } from "./ticket-email-access.ts";

test("email ticket access accepts one URL-safe token from the fragment", () => {
  const token = "A".repeat(43);
  expect(emailAccessToken(`#access_token=${token}`)).toBe(token);
  expect(emailAccessToken(`#access_token=${token}&other=value`)).toBe(token);
  expect(emailAccessToken("#access_token=short")).toBeNull();
  expect(emailAccessToken(`#access_token=${token}&access_token=${token}`)).toBeNull();
  expect(emailAccessToken("#other=value")).toBeNull();
});
