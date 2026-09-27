-- +goose Up
CREATE TABLE recipe_steps (
 recipe_id uuid NOT NULL REFERENCES recipes(id),
 ordinal int NOT NULL CHECK(ordinal BETWEEN 0 AND 63),
 version int NOT NULL DEFAULT 1 CHECK(version>0),
 action text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('prep','cook')),
 duration_seconds int NOT NULL CHECK(duration_seconds BETWEEN 1 AND 28800),
 dependencies int[] NOT NULL DEFAULT '{}',
 equipment text NOT NULL CHECK(equipment IN ('board','stove','rice_cooker','oven')),
 is_parallelizable boolean NOT NULL DEFAULT false,
 source text NOT NULL DEFAULT '项目自编示例，耗时为估计',
 PRIMARY KEY(recipe_id,ordinal)
);
CREATE TABLE cooking_sessions (
 id uuid PRIMARY KEY,
 household_id uuid NOT NULL REFERENCES households(id),
 plan_meal_id uuid NOT NULL REFERENCES plan_meals(id),
 created_by uuid NOT NULL REFERENCES users(id),
 status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','finished','cancelled')),
 target_at timestamptz NOT NULL,
 starts_at timestamptz NOT NULL,
 schedule jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(id,household_id)
);
CREATE UNIQUE INDEX cooking_meal_once ON cooking_sessions(plan_meal_id) WHERE status<>'cancelled';
CREATE INDEX cooking_household_recent ON cooking_sessions(household_id,created_at DESC);
CREATE TABLE cooking_step_progress (
 session_id uuid NOT NULL,
 household_id uuid NOT NULL,
 step_index int NOT NULL CHECK(step_index BETWEEN 0 AND 63),
 started_at timestamptz NOT NULL DEFAULT now(),
 completed_at timestamptz,
 PRIMARY KEY(session_id,step_index),
 FOREIGN KEY(session_id,household_id) REFERENCES cooking_sessions(id,household_id),
 CHECK(completed_at IS NULL OR completed_at>=started_at)
);
CREATE TRIGGER event_cooking AFTER INSERT OR UPDATE ON cooking_sessions FOR EACH ROW EXECUTE FUNCTION publish_foodflow_change();

-- Explicit authored workflows for the four original example recipes.
INSERT INTO recipe_steps(recipe_id,ordinal,action,kind,duration_seconds,dependencies,equipment,is_parallelizable) VALUES
('10000000-0000-4000-8000-000000000001',0,'番茄洗净切块，鸡蛋打散','prep',300,'{}','board',false),
('10000000-0000-4000-8000-000000000001',1,'热锅炒熟鸡蛋盛出','cook',240,'{0}','stove',false),
('10000000-0000-4000-8000-000000000001',2,'炒软番茄，加入鸡蛋翻匀','cook',360,'{1}','stove',false),
('10000000-0000-4000-8000-000000000002',0,'西兰花洗净切小朵','prep',240,'{}','board',false),
('10000000-0000-4000-8000-000000000002',1,'焯水后沥干','cook',180,'{0}','stove',false),
('10000000-0000-4000-8000-000000000002',2,'热锅炒至熟透并调味','cook',300,'{1}','stove',false),
('10000000-0000-4000-8000-000000000003',0,'土豆和鸡肉分别切块，处理生肉后清洗案板和双手','prep',480,'{}','board',false),
('10000000-0000-4000-8000-000000000003',1,'鸡肉下锅翻炒并加入土豆和水','cook',420,'{0}','stove',false),
('10000000-0000-4000-8000-000000000003',2,'小火炖煮，期间留意锅内水量','cook',1320,'{1}','stove',true),
('10000000-0000-4000-8000-000000000003',3,'检查鸡肉熟透、土豆软烂，调味出锅','cook',180,'{2}','stove',false),
('10000000-0000-4000-8000-000000000004',0,'生菜洗净沥干，蒜切末','prep',240,'{}','board',false),
('10000000-0000-4000-8000-000000000004',1,'热锅炒熟并调味','cook',360,'{0}','stove',false);
-- +goose Down
DROP TABLE cooking_step_progress,cooking_sessions,recipe_steps;
