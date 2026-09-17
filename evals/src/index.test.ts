import { describe, expect, it } from "vitest";
import { placeholder } from "./index.js";

describe("evals workspace", () => {
  it("builds and runs (S0 acceptance criterion 4)", () => {
    expect(placeholder).toBe(true);
  });
});
