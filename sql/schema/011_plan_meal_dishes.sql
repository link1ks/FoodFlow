-- +goose Up
-- A meal has one primary recipe in plan_meals and zero or more additional dishes.
CREATE TABLE plan_meal_dishes (
  id uuid PRIMARY KEY,
  plan_meal_id uuid NOT NULL REFERENCES plan_meals(id) ON DELETE CASCADE,
  recipe_id uuid NOT NULL REFERENCES recipes(id),
  UNIQUE(plan_meal_id,recipe_id)
);
CREATE INDEX plan_meal_dishes_meal ON plan_meal_dishes(plan_meal_id);

-- +goose Down
DROP TABLE plan_meal_dishes;
