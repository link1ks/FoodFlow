import { describe, expect, it } from "vitest";
import { monthlyQuantity } from "./monthlyReport";

describe("monthly quantities retain exact accounting units", () => {
  it("does not round milli quantities beyond JavaScript safe integers", () => {
    expect(monthlyQuantity("9007199254740993", "g")).toBe(
      "9007199254740.993 g",
    );
    expect(monthlyQuantity("-1250", "ml")).toBe("-1.25 ml");
    expect(monthlyQuantity("3000", null)).toBe("3 原单位未知");
  });
});
