import { describe, expect, it } from "vitest";
import { shuffled } from "./study";

// A source of "random" numbers that always runs through the same sequence.
function steady() {
  let n = 0;
  const values = [0.1, 0.9, 0.5, 0.3];
  return () => values[n++ % values.length];
}

describe("shuffled", () => {
  it("keeps every item exactly once and leaves the original alone", () => {
    const original = [1, 2, 3, 4, 5, 6];
    const result = shuffled(original);
    expect([...result].sort()).toEqual(original);
    expect(original).toEqual([1, 2, 3, 4, 5, 6]);
  });

  it("gives the same order for the same random numbers", () => {
    const items = ["a", "b", "c", "d", "e"];
    expect(shuffled(items, steady())).toEqual(shuffled(items, steady()));
    expect(shuffled(items, steady())).not.toEqual(items);
  });

  it("handles empty and single lists", () => {
    expect(shuffled([])).toEqual([]);
    expect(shuffled(["only"])).toEqual(["only"]);
  });

  it("actually reorders", () => {
    const seen = new Set<string>();
    for (let i = 0; i < 40; i++) seen.add(shuffled([1, 2, 3, 4]).join(""));
    expect(seen.size).toBeGreaterThan(5);
  });
});
