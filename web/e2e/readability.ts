import { expect, type Locator } from "@playwright/test";

// Sample solid surfaces and all opaque gradient stops used by this UI.
// This checks selected real text/control pairs, not full accessibility compliance.
export async function expectReadable(locator: Locator, minimum = 4.5) {
  const ratio = await locator.evaluate((element) => {
    const rgb = (value: string) => (value.match(/[\d.]+/g) || []).map(Number);
    const luminance = (channels: number[]) => {
      const linear = channels.slice(0, 3).map((channel) => {
        const s = channel / 255;
        return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
      });
      return linear[0] * 0.2126 + linear[1] * 0.7152 + linear[2] * 0.0722;
    };
    const ink = luminance(rgb(getComputedStyle(element).color));
    let node: Element | null = element;
    while (node) {
      const style = getComputedStyle(node);
      const stops = style.backgroundImage.match(/rgba?\([^)]+\)/g);
      const solid = rgb(style.backgroundColor);
      const colors = stops || (solid[3] === 0 ? null : [style.backgroundColor]);
      if (colors) {
        return Math.min(
          ...colors.map((color) => {
            const background = luminance(rgb(color));
            return (
              (Math.max(ink, background) + 0.05) /
              (Math.min(ink, background) + 0.05)
            );
          }),
        );
      }
      node = node.parentElement;
    }
    throw new Error("No opaque surface found for visible text");
  });
  expect(
    ratio,
    "visible text contrast against its actual surface",
  ).toBeGreaterThanOrEqual(minimum);
}
