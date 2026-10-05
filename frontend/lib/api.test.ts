import { AxiosError, AxiosHeaders, type AxiosResponse } from "axios";
import { describe, expect, it } from "vitest";
import { apiError } from "./api";

function httpError(status: number, data: unknown): AxiosError {
  const response = { status, data, statusText: "", headers: {}, config: { headers: new AxiosHeaders() } } as AxiosResponse;
  return new AxiosError("failed", "ERR_BAD_REQUEST", response.config, null, response);
}

describe("apiError", () => {
  it("shows the server's own message", () => {
    expect(apiError(httpError(400, { error: "Password is too common." }), "fallback")).toBe("Password is too common.");
  });

  it("falls back when the server sent no usable message", () => {
    expect(apiError(httpError(500, {}), "fallback")).toBe("fallback");
    expect(apiError(httpError(500, { error: "" }), "fallback")).toBe("fallback");
    expect(apiError(httpError(500, { error: 42 }), "fallback")).toBe("fallback");
  });

  it("explains a network failure", () => {
    const networkError = new AxiosError("Network Error", "ERR_NETWORK");
    expect(apiError(networkError, "fallback")).toMatch(/could not reach the server/i);
  });

  it("uses the fallback for anything that is not an axios error", () => {
    expect(apiError(new Error("boom"), "fallback")).toBe("fallback");
    expect(apiError("text", "fallback")).toBe("fallback");
  });
});
