import type { Recipe } from "./app/shared";

export const recipeCategories = [
  "全部",
  "快手",
  "蔬菜",
  "荤菜",
  "汤",
  "主食",
  "早餐",
] as const;
export function recipeCategory(recipe: Recipe, category: string): boolean {
  if (category === "全部") return true;
  if (category === "快手") return recipe.minutes <= 20;
  if (category === "蔬菜")
    return (
      recipe.tags.includes("蔬菜") ||
      /^(蒜蓉菠菜|清炒小白菜|香菇炒青椒|卷心菜炒胡萝卜|清炒西兰花|蒜香生菜)$/.test(
        recipe.title,
      )
    );
  if (category === "荤菜")
    return recipe.ingredients.some((i) => /肉|虾|鱼/.test(i.name));
  if (category === "主食")
    return recipe.tags.includes("主食") || recipe.title.includes("粥");
  return recipe.tags.includes(category);
}
export function discoverRecipes(
  recipes: Recipe[],
  search: string,
  category: string,
  excluded: string[],
) {
  const query = search.trim().toLocaleLowerCase();
  return recipes.filter(
    (r) =>
      recipeCategory(r, category) &&
      !r.ingredients.some((i) =>
        excluded.some((x) => x.trim() && i.name.includes(x.trim())),
      ) &&
      (!query ||
        [r.title, ...r.tags, ...r.ingredients.map((i) => i.name)]
          .join(" ")
          .toLocaleLowerCase()
          .includes(query)),
  );
}
// Stable, diverse suggestions: one vegetable, one protein dish, then a staple/soup.
// These are reference choices, never claims of available stock or nutritional adequacy.
export function suggestRecipes(
  recipes: Recipe[],
  maxMinutes: number,
  offset: number,
): Recipe[] {
  const eligible = recipes.filter((r) => r.minutes <= maxMinutes);
  if (!eligible.length) return [];
  const shift = offset % eligible.length;
  const rotated = [...eligible.slice(shift), ...eligible.slice(0, shift)];
  const result: Recipe[] = [];
  for (const category of ["蔬菜", "荤菜", "主食"]) {
    const group = eligible.filter(
      (r) => !result.includes(r) && recipeCategory(r, category),
    );
    const match = group[offset % group.length];
    if (match) result.push(match);
  }
  for (const recipe of rotated) {
    if (result.length >= 3) break;
    if (!result.includes(recipe)) result.push(recipe);
  }
  return result;
}
